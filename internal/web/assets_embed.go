//go:build !dev

package web

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"html/template"
	"io"
	"io/fs"
)

//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static
var staticFiles embed.FS

var compiledTemplates = template.Must(
	template.New("").Funcs(baseTemplateFuncs).ParseFS(templateFiles, "templates/*.html"))

func baseTemplates() (*template.Template, error) {
	return compiledTemplates, nil
}

// Static assets can be held for an hour. What makes that safe is assetVersion below: a
// new binary changes the URL, so the browser cannot pair new HTML with old CSS.
const staticCacheControl = "public, max-age=3600"

// assetVersion fingerprints the embedded static tree. Without it, an hour-old app.css
// meets freshly built HTML and the page renders with classes the stylesheet never heard
// of — buttons stretching, inputs unstyled. Seen for real after a UI change.
var assetVersion = fingerprintStatic()

func fingerprintStatic() string {
	h := sha256.New()
	_ = fs.WalkDir(staticFiles, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		f, err := staticFiles.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		io.WriteString(h, path)
		_, _ = io.Copy(h, f)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))[:10]
}

func staticFileSystem() fs.FS {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
