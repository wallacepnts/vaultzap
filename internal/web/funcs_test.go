package web

import "testing"

func TestFileSize(t *testing.T) {
	cases := []struct {
		bytes int64
		want  string
	}{
		{0, ""},
		{-1, ""},
		{512, "512 B"},
		{1024, "1 KB"},
		{218112, "213 KB"},
		{131072000, "125 MB"},
	}
	for _, c := range cases {
		if got := fileSize(c.bytes); got != c.want {
			t.Errorf("fileSize(%d) = %q, esperado %q", c.bytes, got, c.want)
		}
	}
}

func TestFileExtension(t *testing.T) {
	cases := map[string]string{
		"doc.pdf":      "PDF",
		"IMG_0833.MP4": "MP4",
		"contato.vcf":  "VCF",
		"sem_extensao": "ARQ",
	}
	for name, want := range cases {
		if got := fileExtension(name); got != want {
			t.Errorf("fileExtension(%q) = %q, esperado %q", name, got, want)
		}
	}
}

func TestEscapeNonASCII(t *testing.T) {
	cases := []struct{ input, want string }{
		{`{"texto":"ok"}`, `{"texto":"ok"}`},
		{`{"texto":"Você"}`, `{"texto":"Voc\u00ea"}`},
		{`{"texto":"até 3"}`, `{"texto":"at\u00e9 3"}`},
		{`{"texto":"🫶"}`, `{"texto":"\ud83e\udef6"}`},
	}
	for _, c := range cases {
		if got := escapeNonASCII(c.input); got != c.want {
			t.Errorf("escapeNonASCII(%q) = %q, esperado %q", c.input, got, c.want)
		}
	}
}
