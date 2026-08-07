package web

import (
	"encoding/xml"
	"io"
	"io/fs"
	"path"
	"strings"
	"testing"
)

// A standalone .svg is parsed as strict XML, unlike the same markup inlined in HTML: a
// "--" inside a comment makes the whole file invalid and the favicon silently vanishes.
func TestSVGsAreValidXML(t *testing.T) {
	assets := staticFileSystem()

	var found int
	err := fs.WalkDir(assets, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.EqualFold(path.Ext(name), ".svg") {
			return nil
		}
		found++

		file, err := assets.Open(name)
		if err != nil {
			return err
		}
		defer file.Close()

		decoder := xml.NewDecoder(file)
		for {
			if _, err := decoder.Token(); err == io.EOF {
				return nil
			} else if err != nil {
				t.Errorf("%s: XML inválido, não vai renderizar como favicon/<img>: %v", name, err)
				return nil
			}
		}
	})
	if err != nil {
		t.Fatalf("percorrer assets: %v", err)
	}
	if found == 0 {
		t.Fatal("nenhum .svg encontrado; o teste não está olhando onde deveria")
	}
}

// The browser caches static assets for an hour. Without a token in the URL, a new binary
// serves fresh HTML that a stale stylesheet knows nothing about — the layout falls apart
// with no error anywhere. Happened for real after a UI change.
func TestAssetURLCarriesVersion(t *testing.T) {
	url := asset("css/app.css")
	if !strings.HasPrefix(url, "/static/css/app.css?v=") {
		t.Fatalf("asset = %q, queria /static/css/app.css?v=...", url)
	}
	if len(assetVersion) < 6 {
		t.Errorf("versão curta demais: %q", assetVersion)
	}
}
