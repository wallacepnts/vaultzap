package locale

import (
	"testing"

	"github.com/wallacepnts/vaultzap/internal/render"
)

func TestDecodeCatalog(t *testing.T) {
	input := `# comment
# another comment

msgid "key.simple"
msgstr "Simple text"

msgid "key.with_quotes"
msgstr "She said \"hi\" and left"

msgid "key.with_backslash"
msgstr "C:\\path\\file"

msgid "key.continuation"
msgstr "First part "
"and second part"

msgid "key.format"
msgstr "%d messages from %s"
`
	catalog, err := decodeCatalog([]byte(input))
	if err != nil {
		t.Fatalf("decodeCatalog: %v", err)
	}

	cases := map[string]string{
		"key.simple":         "Simple text",
		"key.with_quotes":    `She said "hi" and left`,
		"key.with_backslash": `C:\path\file`,
		"key.continuation":   "First part and second part",
		"key.format":         "%d messages from %s",
	}
	for key, expected := range cases {
		if got := catalog[key]; got != expected {
			t.Errorf("catalog[%q] = %q, expected %q", key, got, expected)
		}
	}
	if len(catalog) != len(cases) {
		t.Errorf("catalog has %d entries, expected %d", len(catalog), len(cases))
	}
}

func TestDecodeCatalog_msgstrWithoutMsgid(t *testing.T) {
	if _, err := decodeCatalog([]byte(`msgstr "solto"`)); err == nil {
		t.Error("expected an error for msgstr without a preceding msgid")
	}
}

func TestRealCatalogs(t *testing.T) {
	catalogs, err := loadCatalogs()
	if err != nil {
		t.Fatalf("loadCatalogs: %v", err)
	}
	if len(catalogs) != len(render.Locales) {
		t.Fatalf("expected %d loaded languages, got %d", len(render.Locales), len(catalogs))
	}
	for lang, catalog := range catalogs {
		if len(catalog) == 0 {
			t.Errorf("catalog %s came back empty", lang)
		}
	}
}
