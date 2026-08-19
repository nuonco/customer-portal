package spa

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const indexHTML = `<!doctype html><html><head><title>t</title></head><body><div id="root"></div></body></html>`

// newTestHandler builds a router with a populated dist dir and one pre-existing
// API route, mirroring how main.go registers the SPA last.
func newTestHandler(t *testing.T) *gin.Engine {
	t.Helper()

	dist := t.TempDir()
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte(indexHTML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "favicon.svg"), []byte("<svg/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dist, "assets"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "assets", "app-abc123.js"), []byte("console.log(1)"), 0o600); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.GET("/portal-api/state", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	h := NewHandler(Config{
		DistDir:             dist,
		SubdomainBaseDomain: "installs.example.com",
		NuonAPIURL:          "http://localhost:8081",
	}, zap.NewNop())
	if err := h.RegisterRoutes(e); err != nil {
		t.Fatal(err)
	}
	return e
}

func do(t *testing.T, e *gin.Engine, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	return w
}

func TestExplicitRoutesTakePrecedence(t *testing.T) {
	e := newTestHandler(t)
	w := do(t, e, http.MethodGet, "/portal-api/state", nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("registered API route was shadowed by the SPA fallback: %d %s", w.Code, w.Body.String())
	}
}

func TestDeepLinkServesIndexWithInjectedConfig(t *testing.T) {
	e := newTestHandler(t)
	w := do(t, e, http.MethodGet, "/installs/inst_123/stack", map[string]string{
		"Accept": "text/html,application/xhtml+xml",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for client-side route, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `id="root"`) {
		t.Error("expected index.html to be served")
	}
	if !strings.Contains(body, `window.__PORTAL_CONFIG__=`) {
		t.Error("runtime config was not injected")
	}
	if !strings.Contains(body, `"subdomainBaseDomain":"installs.example.com"`) {
		t.Errorf("config missing subdomainBaseDomain: %s", body)
	}
	// The client shows this as the Connect Org placeholder, so a blank field
	// advertises the URL the server will actually use.
	if !strings.Contains(body, `"nuonApiUrl":"http://localhost:8081"`) {
		t.Errorf("config missing nuonApiUrl: %s", body)
	}
	// The config lands just before </head>, which is *after* Vite's
	// <script type="module"> tag. That is still correct: module scripts are
	// deferred until parsing completes, whereas this classic inline script runs
	// immediately, so the config is always set before the app boots.
	if strings.Index(body, "__PORTAL_CONFIG__") > strings.Index(body, "</head>") {
		t.Error("config script must be injected before </head>")
	}
	if !strings.Contains(body, `<script id="portal-config">`) {
		t.Error("config must be a classic (non-deferred) inline script")
	}
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Errorf("index.html must not be cached, got %q", cc)
	}
}

func TestUnknownAPIPathReturnsJSONNotHTML(t *testing.T) {
	// A 200 with an HTML body here would surface as a JSON parse error in the
	// client, hiding the real problem.
	for _, path := range []string{
		"/portal-api/nope",
		"/auth-api/nope",
		"/admin/orgs/org_1/apps-api/nope",
		"/admin/orgs/org_1/installs-api",
	} {
		t.Run(path, func(t *testing.T) {
			w := do(t, newTestHandler(t), http.MethodGet, path, nil)
			if w.Code != http.StatusNotFound {
				t.Errorf("expected 404, got %d", w.Code)
			}
			if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
				t.Errorf("expected JSON content type, got %q", ct)
			}
		})
	}
}

func TestJSONAcceptHeaderNeverGetsHTML(t *testing.T) {
	w := do(t, newTestHandler(t), http.MethodGet, "/some/unknown/path", map[string]string{
		"Accept": "application/json",
	})
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for a JSON request, got %d", w.Code)
	}
}

func TestStaticFilesAreServedAndCached(t *testing.T) {
	e := newTestHandler(t)

	w := do(t, e, http.MethodGet, "/assets/app-abc123.js", nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "console.log") {
		t.Fatalf("hashed asset not served: %d", w.Code)
	}
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("hashed assets should be immutable, got %q", cc)
	}

	// Root-level files that are not client-side routes come from dist too.
	if w := do(t, e, http.MethodGet, "/favicon.svg", nil); w.Code != http.StatusOK {
		t.Errorf("expected favicon to be served, got %d", w.Code)
	}
}

func TestNonGETDoesNotFallBackToIndex(t *testing.T) {
	w := do(t, newTestHandler(t), http.MethodPost, "/installs/inst_123", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unhandled POST, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "id=\"root\"") {
		t.Error("POST must not receive the SPA shell")
	}
}

func TestMissingDistDirDoesNotBreakAPIRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.GET("/livez", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	h := NewHandler(Config{DistDir: filepath.Join(t.TempDir(), "absent")}, zap.NewNop())
	if err := h.RegisterRoutes(e); err != nil {
		t.Fatalf("a missing dist dir must not be fatal: %v", err)
	}

	if w := do(t, e, http.MethodGet, "/livez", nil); w.Code != http.StatusOK {
		t.Errorf("health check broken by missing dist dir: %d", w.Code)
	}
	if w := do(t, e, http.MethodGet, "/installs", nil); w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when the UI is unavailable, got %d", w.Code)
	}
}

// TestBundleAppearingAfterStartupIsServed is the regression test for a real bug:
// presence of the bundle was cached at RegisterRoutes time, so in development —
// where the Go server routinely starts before `vite build --watch` finishes — the
// UI returned 503 forever until the server was restarted.
func TestBundleAppearingAfterStartupIsServed(t *testing.T) {
	dist := t.TempDir()

	gin.SetMode(gin.TestMode)
	e := gin.New()
	h := NewHandler(Config{DistDir: dist, SubdomainBaseDomain: "installs.example.com"}, zap.NewNop())
	if err := h.RegisterRoutes(e); err != nil {
		t.Fatal(err)
	}

	// Startup state: no bundle yet.
	if w := do(t, e, http.MethodGet, "/installs", map[string]string{"Accept": "text/html"}); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 before the bundle exists, got %d", w.Code)
	}

	// The build lands while the server is still running.
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte(indexHTML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dist, "assets"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "assets", "app-abc123.js"), []byte("console.log(1)"), 0o600); err != nil {
		t.Fatal(err)
	}

	w := do(t, e, http.MethodGet, "/installs", map[string]string{"Accept": "text/html"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected the UI to become available without a restart, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `id="root"`) {
		t.Error("expected index.html to be served once the bundle appeared")
	}
	if !strings.Contains(w.Body.String(), "__PORTAL_CONFIG__") {
		t.Error("runtime config must still be injected after late discovery")
	}

	// /assets must be routed even though it did not exist at registration time.
	if a := do(t, e, http.MethodGet, "/assets/app-abc123.js", nil); a.Code != http.StatusOK {
		t.Errorf("expected /assets to be served after late discovery, got %d", a.Code)
	}
}

func TestNoCacheAssetsForLocalDev(t *testing.T) {
	dist := t.TempDir()
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte(indexHTML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dist, "assets"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "assets", "app.js"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	e := gin.New()
	h := NewHandler(Config{DistDir: dist, NoCacheAssets: true}, zap.NewNop())
	if err := h.RegisterRoutes(e); err != nil {
		t.Fatal(err)
	}

	w := do(t, e, http.MethodGet, "/assets/app.js", nil)
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("expected no-store in local dev, got %q", cc)
	}
}

func TestIsAPIPath(t *testing.T) {
	api := []string{
		"/portal-api/state",
		"/auth-api/session",
		"/admin/orgs/org_1/apps-api",
		"/admin/orgs/org_1/install-links-api/link_1",
		"/admin/superuser-api/orgs/search",
	}
	notAPI := []string{
		"/",
		"/installs",
		"/installs/inst_1/stack",
		"/admin/orgs/org_1/apps/app_1",
		"/account",
		"/api-docs",
	}

	for _, p := range api {
		if !isAPIPath(p) {
			t.Errorf("expected %q to be treated as an API path", p)
		}
	}
	for _, p := range notAPI {
		if isAPIPath(p) {
			t.Errorf("expected %q to be treated as a client-side route", p)
		}
	}
}

func TestPrefersJSON(t *testing.T) {
	cases := map[string]bool{
		"application/json":                     true,
		"application/json, text/plain, */*":    true,
		"text/html,application/xhtml+xml":      false,
		"text/html,application/json":           false, // browser navigation
		"":                                     false,
		"text/html,application/json;q=0.9,*/*": false,
	}
	for accept, want := range cases {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
		c.Request.Header.Set("Accept", accept)
		if got := prefersJSON(c); got != want {
			t.Errorf("prefersJSON(%q) = %v, want %v", accept, got, want)
		}
	}
}

func TestDevReloadInjectsScriptAndExposesVersion(t *testing.T) {
	dist := t.TempDir()
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte(indexHTML), 0o600); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	e := gin.New()
	h := NewHandler(Config{DistDir: dist, DevReload: true}, zap.NewNop())
	if err := h.RegisterRoutes(e); err != nil {
		t.Fatal(err)
	}

	w := do(t, e, http.MethodGet, "/installs", map[string]string{"Accept": "text/html"})
	if !strings.Contains(w.Body.String(), DistVersionPath) {
		t.Error("expected the dev reload script to be injected")
	}

	first := do(t, e, http.MethodGet, DistVersionPath, nil)
	if first.Code != http.StatusOK || first.Body.Len() == 0 {
		t.Fatalf("expected a dist signature, got %d %q", first.Code, first.Body.String())
	}

	// A rebuild must change the signature so the browser reloads.
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte(indexHTML+"<!--rebuilt-->"), 0o600); err != nil {
		t.Fatal(err)
	}
	second := do(t, e, http.MethodGet, DistVersionPath, nil)
	if second.Body.String() == first.Body.String() {
		t.Error("expected the dist signature to change after a rebuild")
	}
}

func TestDevReloadDisabledByDefault(t *testing.T) {
	e := newTestHandler(t)

	w := do(t, e, http.MethodGet, "/installs", map[string]string{"Accept": "text/html"})
	if strings.Contains(w.Body.String(), DistVersionPath) {
		t.Error("the reload script must not ship in production builds")
	}
	// The endpoint is simply never registered, so it falls through to the SPA
	// shell like any other unknown path. What matters is that it does not serve
	// a dist signature that something could come to depend on.
	got := do(t, e, http.MethodGet, DistVersionPath, map[string]string{"Accept": "text/html"})
	if !strings.Contains(got.Body.String(), `id="root"`) {
		t.Errorf("expected %s to fall through to the SPA shell, got %q", DistVersionPath, got.Body.String())
	}
}

// Files copied verbatim from public/ (fonts, favicons) keep the same name across
// rebuilds, so pinning them for a year would make a replacement invisible to
// anyone who had already loaded the old one. Only hashed /assets get that.
func TestUnhashedPublicFilesAreNotCachedForever(t *testing.T) {
	dist := t.TempDir()
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte(indexHTML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dist, "fonts"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "fonts", "hack-regular.woff2"), []byte("font"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dist, "assets"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "assets", "app-abc123.js"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	e := gin.New()
	h := NewHandler(Config{DistDir: dist}, zap.NewNop())
	if err := h.RegisterRoutes(e); err != nil {
		t.Fatal(err)
	}

	font := do(t, e, http.MethodGet, "/fonts/hack-regular.woff2", nil)
	if font.Code != http.StatusOK {
		t.Fatalf("expected the font to be served, got %d", font.Code)
	}
	if cc := font.Header().Get("Cache-Control"); strings.Contains(cc, "immutable") {
		t.Errorf("unhashed public files must not be immutable, got %q", cc)
	}

	asset := do(t, e, http.MethodGet, "/assets/app-abc123.js", nil)
	if cc := asset.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("hashed assets should stay immutable, got %q", cc)
	}
}
