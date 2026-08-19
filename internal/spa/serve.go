// Package spa serves the React single-page application from a Vite build
// directory.
//
// Two things make this work in production the same way it works locally:
//
//  1. Client-side routing needs every unrecognized GET to return index.html so
//     that deep links (e.g. /installs/inst_123/stack) survive a hard refresh.
//     That is implemented with gin's NoRoute, which means RegisterRoutes MUST be
//     called after every other route is registered — see RegisterRoutes.
//
//  2. The client needs environment-specific values (chiefly the subdomain base
//     domain) that differ between stage and prod. Baking them into the bundle at
//     build time would require one image per environment, so instead the server
//     injects them into index.html at request time as window.__PORTAL_CONFIG__.
package spa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Config holds the values needed to serve the SPA. Everything is passed
// explicitly rather than read from the environment here so that main.go stays
// the single place environment variables are resolved.
type Config struct {
	// DistDir is the Vite build output directory.
	DistDir string

	// SubdomainBaseDomain is the domain org subdomains hang off of, e.g.
	// "installs.nuon.co". The client subtracts it from window.location.host to
	// determine which org's portal it is rendering.
	SubdomainBaseDomain string

	// CustomerSubdomain is a development-only fallback used when the app is
	// reached on a host with no subdomain. Empty in production.
	CustomerSubdomain string

	// NuonAPIURL is the upstream Nuon API the server talks to. Surfaced to the
	// client so forms can show the URL that a blank field will actually use,
	// rather than a hardcoded guess (see ConnectOrgModal).
	NuonAPIURL string

	Version string
	GitRef  string

	// NoCacheAssets disables long-lived asset caching. Set in local development,
	// where the bundle is rebuilt constantly and filenames are not hashed.
	NoCacheAssets bool

	// DevReload injects a script that reloads the page when the bundle is
	// rebuilt, and exposes DistVersionPath for it to poll. Local development
	// only.
	DevReload bool
}

// clientConfig is the payload serialized into window.__PORTAL_CONFIG__. Field
// names are camelCase to read naturally from TypeScript.
type clientConfig struct {
	SubdomainBaseDomain string `json:"subdomainBaseDomain,omitempty"`
	CustomerSubdomain   string `json:"customerSubdomain,omitempty"`
	NuonAPIURL          string `json:"nuonApiUrl,omitempty"`
	Version             string `json:"version,omitempty"`
	GitRef              string `json:"gitRef,omitempty"`
}

// apiPrefixes are paths that must never fall back to index.html. Without this a
// typo'd or unauthenticated API call would return a 200 with an HTML body, and
// the client would fail while trying to parse it as JSON — a confusing failure
// to debug. Returning a JSON 404 surfaces the real problem instead.
var apiPrefixes = []string{
	"/portal-api/",
	"/auth-api/",
}

type Handler struct {
	cfg Config
	l   *zap.Logger
}

func NewHandler(cfg Config, l *zap.Logger) *Handler {
	if cfg.DistDir == "" {
		cfg.DistDir = "./ui/dist"
	}
	return &Handler{cfg: cfg, l: l}
}

// RegisterRoutes wires up asset serving and the client-side routing fallback.
//
// This MUST be called after all other routes are registered. It installs a
// NoRoute catch-all, so any route registered afterwards still works, but any
// path this handler is expected to leave alone must already be registered.
func (h *Handler) RegisterRoutes(e *gin.Engine) error {
	distFS := os.DirFS(h.cfg.DistDir)

	// A missing dist directory is not fatal: the JSON APIs and health checks
	// remain useful, and failing to boot would turn a stale build into an
	// outage. Log loudly instead.
	//
	// Crucially this is only a warning, never cached state. In development the Go
	// server routinely starts before `vite build --watch` has produced a bundle,
	// so presence is re-checked per request — otherwise the UI would stay
	// unavailable until the next server restart even once the build landed.
	if _, err := fs.Stat(distFS, "index.html"); err != nil {
		h.l.Warn("SPA bundle not present yet — the React UI will 503 until it is built",
			zap.String("dist_dir", h.cfg.DistDir), zap.Error(err))
	}

	ccJSON, err := json.Marshal(clientConfig{
		SubdomainBaseDomain: h.cfg.SubdomainBaseDomain,
		CustomerSubdomain:   h.cfg.CustomerSubdomain,
		NuonAPIURL:          h.cfg.NuonAPIURL,
		Version:             h.cfg.Version,
		GitRef:              h.cfg.GitRef,
	})
	if err != nil {
		return fmt.Errorf("unable to marshal client config: %w", err)
	}
	injected := fmt.Sprintf(
		`<script id="portal-config">window.__PORTAL_CONFIG__=%s;</script>`, ccJSON)
	if h.cfg.DevReload {
		injected += reloadScript
		registerDevReload(e, distFS)
	}
	configScript := []byte(injected)

	// Vite content-hashes everything under /assets, so those are safe to pin for
	// a year. Files copied verbatim from public/ (fonts, favicons) keep a stable
	// name across rebuilds, so they get a short TTL instead — otherwise replacing
	// a font would be invisible to anyone who had already loaded it.
	cacheControl := "public, max-age=31536000, immutable"
	staticCacheControl := "public, max-age=3600"
	if h.cfg.NoCacheAssets {
		cacheControl = "no-cache, no-store, must-revalidate"
		staticCacheControl = cacheControl
	}

	fileServer := http.FileServer(http.FS(distFS))

	// Registered unconditionally: the bundle may not exist yet at startup.
	// Vite emits hashed filenames under /assets, so these are safe to cache for a
	// year. index.html is deliberately not cached (see serveIndex).
	e.GET("/assets/*filepath", func(c *gin.Context) {
		c.Header("Cache-Control", cacheControl)
		fileServer.ServeHTTP(c.Writer, c.Request)
	})

	serveIndex := func(c *gin.Context) {
		raw, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			h.l.Error("unable to read SPA index.html",
				zap.String("dist_dir", h.cfg.DistDir), zap.Error(err))
			c.String(http.StatusServiceUnavailable, "UI unavailable — bundle not built yet")
			return
		}
		// Inject before </head> so the config is set before the bundle runs.
		html := bytes.Replace(raw, []byte("</head>"), append(configScript, []byte("</head>")...), 1)
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Data(http.StatusOK, "text/html; charset=utf-8", html)
	}

	e.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		if isAPIPath(path) || prefersJSON(c) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// Only GET can be a client-side route. Anything else reaching NoRoute is
		// a genuinely unhandled request.
		if c.Request.Method != http.MethodGet {
			c.Status(http.StatusNotFound)
			return
		}

		// Serve a real file if one exists (favicon, manifest, etc.), otherwise
		// hand the path to the client-side router.
		if file := strings.TrimPrefix(path, "/"); file != "" && file != "index.html" {
			if stat, err := fs.Stat(distFS, file); err == nil && !stat.IsDir() {
				c.Header("Cache-Control", staticCacheControl)
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}

		serveIndex(c)
	})

	h.l.Info("registered SPA routes",
		zap.String("dist_dir", h.cfg.DistDir),
		zap.String("subdomain_base_domain", h.cfg.SubdomainBaseDomain))

	return nil
}

func isAPIPath(path string) bool {
	for _, prefix := range apiPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	// Vendor JSON endpoints are named with an "-api" suffix on the resource
	// segment (e.g. /admin/orgs/:org_id/apps-api). They live under /admin, which
	// is also a client-side route prefix, so they cannot be matched by prefix.
	for _, seg := range strings.Split(path, "/") {
		if strings.HasSuffix(seg, "-api") {
			return true
		}
	}
	return false
}

// prefersJSON reports whether the caller asked for JSON. Every fetch in the
// client sets this, while browser navigations ask for text/html, so it cleanly
// separates a broken API call from a client-side route.
func prefersJSON(c *gin.Context) bool {
	accept := c.GetHeader("Accept")
	return strings.Contains(accept, "application/json") &&
		!strings.Contains(accept, "text/html")
}
