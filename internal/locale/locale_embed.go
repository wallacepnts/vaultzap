//go:build !dev

package locale

import (
	"embed"

	"github.com/wallacepnts/vaultzap/internal/render"
)

//go:embed pt-BR.po pt.po en.po es.po it.po fr.po de.po nl.po
var catalogFiles embed.FS

var compiledCatalogs = loadEmbeddedCatalogs()

func loadEmbeddedCatalogs() map[render.Locale]map[string]string {
	catalogs := make(map[render.Locale]map[string]string, len(render.Locales))
	for _, lang := range render.Locales {
		data, err := catalogFiles.ReadFile(string(lang) + ".po")
		if err != nil {
			panic(err) // packaging error (go:embed), not a runtime one
		}
		catalog, err := decodeCatalog(data)
		if err != nil {
			panic(err)
		}
		catalogs[lang] = catalog
	}
	return catalogs
}

func loadCatalogs() (map[render.Locale]map[string]string, error) {
	return compiledCatalogs, nil
}
