//go:build dev

package locale

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/wallacepnts/vaultzap/internal/render"
)

var baseDir = func() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}()

func loadCatalogs() (map[render.Locale]map[string]string, error) {
	catalogs := make(map[render.Locale]map[string]string, len(render.Locales))
	for _, lang := range render.Locales {
		data, err := os.ReadFile(filepath.Join(baseDir, string(lang)+".po"))
		if err != nil {
			return nil, err
		}
		catalog, err := decodeCatalog(data)
		if err != nil {
			return nil, err
		}
		catalogs[lang] = catalog
	}
	return catalogs, nil
}
