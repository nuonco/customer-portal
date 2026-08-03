package assets

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

// hashedName matches the "<base>.<8hex>.<ext>" URLs produced by Path.
var hashedName = regexp.MustCompile(`^/static/css/customer\.[a-f0-9]{8}\.css$`)

func writeCSS(t *testing.T, dir, contents string) {
	t.Helper()
	cssDir := filepath.Join(dir, "css")
	if err := os.MkdirAll(cssDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cssDir, "customer.css"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPath_HashesOnDiskFile(t *testing.T) {
	dir := t.TempDir()
	writeCSS(t, dir, "body{color:red}")
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}

	got := Path("css/customer.css")
	if !hashedName.MatchString(got) {
		t.Fatalf("Path() = %q, want a content-hashed /static/css/customer.<hash>.css", got)
	}
}

func TestPath_ChangesWhenContentChanges(t *testing.T) {
	dir := t.TempDir()
	writeCSS(t, dir, "body{color:red}")
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	first := Path("css/customer.css")

	// Rewrite with different content and a newer mtime; the hash must update
	// so browsers bust their cache. This is the drift scenario that previously
	// broke: the served file changed but the URL did not (or vice versa).
	writeCSS(t, dir, "body{color:blue}")
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "css", "customer.css"), later, later); err != nil {
		t.Fatal(err)
	}

	second := Path("css/customer.css")
	if first == second {
		t.Fatalf("Path() did not change after content changed: still %q", second)
	}
}

func TestPath_FallsBackWhenMissing(t *testing.T) {
	if err := Init(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	// No file on disk: return the plain path rather than a broken hashed URL.
	if got := Path("css/customer.css"); got != "/static/css/customer.css" {
		t.Fatalf("Path() = %q, want plain fallback /static/css/customer.css", got)
	}
}
