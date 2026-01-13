package assets

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Manifest maps original asset names to their hashed versions
type Manifest struct {
	mu     sync.RWMutex
	assets map[string]string // e.g., "css/vendor.css" -> "css/vendor.a1b2c3d4.css"
}

var (
	globalManifest *Manifest
	once           sync.Once
)

// Init initializes the global asset manifest
// Call this at application startup
func Init(staticDir string) error {
	var initErr error
	once.Do(func() {
		globalManifest = &Manifest{
			assets: make(map[string]string),
		}
		initErr = globalManifest.load(staticDir)
	})
	return initErr
}

// Path returns the cache-busted path for an asset
// e.g., Path("css/vendor.css") -> "/static/css/vendor.a1b2c3d4.css"
func Path(name string) string {
	if globalManifest == nil {
		// Fallback if not initialized
		return "/static/" + name
	}
	globalManifest.mu.RLock()
	defer globalManifest.mu.RUnlock()

	if hashed, ok := globalManifest.assets[name]; ok {
		return "/static/" + hashed
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

// load reads the manifest.json file
func (m *Manifest) load(staticDir string) error {
	manifestPath := filepath.Join(staticDir, "manifest.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No manifest file - development mode, compute hashes on the fly
			return m.computeHashes(staticDir)
		}
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	return json.Unmarshal(data, &m.assets)
}

// computeHashes computes hashes for all CSS files (fallback for dev mode)
func (m *Manifest) computeHashes(staticDir string) error {
	cssFiles := []string{"css/vendor.css", "css/customer.css"}

	for _, cssFile := range cssFiles {
		fullPath := filepath.Join(staticDir, cssFile)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			continue
		}

		hash, err := computeFileHash(fullPath)
		if err != nil {
			continue
		}

		// Store mapping: "css/vendor.css" -> "css/vendor.a1b2c3d4.css"
		ext := filepath.Ext(cssFile)
		base := strings.TrimSuffix(cssFile, ext)
		m.assets[cssFile] = fmt.Sprintf("%s.%s%s", base, hash[:8], ext)
	}

	return nil
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
