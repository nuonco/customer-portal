package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newAlertContext(t *testing.T, accept, hxRequest string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/installs/inst_1/reprovision", nil)
	if accept != "" {
		c.Request.Header.Set("Accept", accept)
	}
	if hxRequest != "" {
		c.Request.Header.Set("HX-Request", hxRequest)
	}
	return c, w
}

// The React client follows a 302 transparently, so redirecting it landed on the
// SPA fallback and every outcome read as "Failed to ... (404)".
func TestAlertResponseAnswersJSONClientsWithJSON(t *testing.T) {
	for _, tc := range []struct {
		name       string
		alertType  string
		alertMsg   string
		wantStatus int
	}{
		{"success", "success", "Install reprovisioning initiated successfully", http.StatusOK},
		{"error", "error", "Failed to reprovision install: install is not active", http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{}
			c, w := newAlertContext(t, "application/json", "")

			h.alertResponse(c, tc.alertType, tc.alertMsg, "/installs/inst_1/overview?alert_type=x")

			if w.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tc.wantStatus)
			}
			if loc := w.Header().Get("Location"); loc != "" {
				t.Errorf("must not redirect a JSON client, got Location %q", loc)
			}

			var body struct {
				Status  string `json:"status"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("expected a JSON body, got %q: %v", w.Body.String(), err)
			}
			if body.Status != tc.alertType {
				t.Errorf("status field = %q, want %q", body.Status, tc.alertType)
			}
			// The message is the whole point: it carries the real cause.
			if body.Message != tc.alertMsg {
				t.Errorf("message = %q, want %q", body.Message, tc.alertMsg)
			}
		})
	}
}

func TestAlertResponseStillRedirectsBrowsers(t *testing.T) {
	h := &Handler{}
	c, w := newAlertContext(t, "text/html,application/xhtml+xml", "")

	h.alertResponse(c, "success", "done", "/installs/inst_1/overview?alert_type=success")

	// gin buffers the status until the header is flushed; c.JSON flushes as a side
	// effect of writing a body, but c.Redirect does not.
	c.Writer.WriteHeaderNow()

	if w.Code != http.StatusFound {
		t.Errorf("expected a 302 for a browser navigation, got %d", w.Code)
	}
	if got := w.Header().Get("Location"); got != "/installs/inst_1/overview?alert_type=success" {
		t.Errorf("Location = %q", got)
	}
}

func TestAlertResponseStillUsesHXRedirectForHTMX(t *testing.T) {
	h := &Handler{}
	c, w := newAlertContext(t, "text/html", "true")

	h.alertResponse(c, "success", "done", "/installs/inst_1/overview")
	c.Writer.WriteHeaderNow()

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for HTMX, got %d", w.Code)
	}
	if got := w.Header().Get("HX-Redirect"); got != "/installs/inst_1/overview" {
		t.Errorf("HX-Redirect = %q", got)
	}
}

func TestWantsJSON(t *testing.T) {
	cases := map[string]bool{
		"application/json":                  true,
		"application/json, text/plain, */*": true,
		"text/html,application/xhtml+xml":   false,
		// A browser navigation that also accepts JSON must not be treated as an
		// API call, or hx-boost and plain links would stop redirecting.
		"text/html,application/json": false,
		"":                           false,
	}
	for accept, want := range cases {
		c, _ := newAlertContext(t, accept, "")
		if got := wantsJSON(c); got != want {
			t.Errorf("wantsJSON(%q) = %v, want %v", accept, got, want)
		}
	}
}
