//go:build dev

package web

import (
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

var baseDir = func() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}()

func baseTemplates() (*template.Template, error) {
	pattern := filepath.Join(baseDir, "templates", "*.html")
	return template.New("").Funcs(baseTemplateFuncs).ParseGlob(pattern)
}

// Every start gets its own token; with no-cache below it changes nothing in practice, but
// it keeps the template contract identical between the two builds.
var assetVersion = strconv.FormatInt(time.Now().UnixNano(), 36)

// Makes the browser revalidate every static asset, so an edit shows on reload.
const staticCacheControl = "no-cache"

func staticFileSystem() fs.FS {
	return os.DirFS(filepath.Join(baseDir, "static"))
}
