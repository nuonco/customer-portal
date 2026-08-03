// Package assets provides cache-busted URLs for static assets.
//
// The cache-busting token is the content hash of the file *on disk*, computed
// lazily and re-derived whenever the file's mtime changes.
package assets

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	mu        sync.RWMutex
	staticDir = "./static"
	cache     = map[string]cacheEntry{} // asset name -> hash keyed by mtime
)

type cacheEntry struct {
	modTime int64
	hash    string
}

// Init records the directory static assets are served from.
func Init(dir string) error {
	mu.Lock()
	staticDir = dir
	cache = map[string]cacheEntry{}
	mu.Unlock()
	return nil
}

// Path returns a cache-busted URL for an asset, e.g.
// Path("css/customer.css") -> "/static/css/customer.6491ba3f.css".
// When the file cannot be read the plain path is returned as a fallback.
func Path(name string) string {
	if hash := version(name); hash != "" {
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		return "/static/" + base + "." + hash + ext
	}
	return "/static/" + name
}

// VendorCSSPath returns the cache-busted path for vendor.css
func VendorCSSPath() string {
	return Path("css/vendor.css")
}

// CustomerCSSPath returns the cache-busted path for customer.css
func CustomerCSSPath() string {
	return Path("css/customer.css")
}

// version returns the 8-char content hash for an on-disk asset, recomputing it
// only when the file's mtime changes so repeated calls are cheap in production
// while still picking up rebuilds in dev.
func version(name string) string {
	mu.RLock()
	dir := staticDir
	entry, ok := cache[name]
	mu.RUnlock()

	fullPath := filepath.Join(dir, name)
	fi, err := os.Stat(fullPath)
	if err != nil {
		return ""
	}
	mt := fi.ModTime().UnixNano()
	if ok && entry.modTime == mt {
		return entry.hash
	}

	hash, err := computeFileHash(fullPath)
	if err != nil {
		return ""
	}
	short := hash[:8]

	mu.Lock()
	cache[name] = cacheEntry{modTime: mt, hash: short}
	mu.Unlock()
	return short
}

func computeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
