package spa

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

// DistVersionPath is polled by the injected dev reload script. It is distinct
// from main.go's /dev/version, which only changes when the Go server restarts
// and so cannot detect a client-only rebuild.
const DistVersionPath = "/dev/dist-version"

// reloadScript polls the dist signature and reloads when the bundle changes.
//
// Deliberately same-origin: dashboard-ui runs an equivalent reload proxy on a
// separate port, but a second origin in dev would resurrect the cookie and
// subdomain problems that the ui_port workaround exists to paper over.
const reloadScript = `<script>(function(){
var current=null;
function poll(){
  fetch("` + DistVersionPath + `",{cache:"no-store"})
    .then(function(r){return r.text()})
    .then(function(v){
      if(current===null){current=v;return}
      if(v!==current){location.reload()}
    })
    .catch(function(){/* server restarting; keep polling */});
}
setInterval(poll,1000);
poll();
})();</script>`

// distSignature hashes the name, size, and modification time of every file in
// the build directory. Vite rewrites hashed filenames on each build, so this
// changes on any rebuild.
func distSignature(distFS fs.FS) string {
	h := sha256.New()
	err := fs.WalkDir(distFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			// A file removed mid-walk is normal during a rebuild; skip it.
			return nil //nolint:nilerr // transient during rebuild
		}
		fmt.Fprintf(h, "%s:%d:%d\n", path, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// registerDevReload exposes the dist signature endpoint used by reloadScript.
func registerDevReload(e *gin.Engine, distFS fs.FS) {
	e.GET(DistVersionPath, func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.String(http.StatusOK, distSignature(distFS))
	})
}
