package parser

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files from the parser's current output")

func TestChatNameFromFile(t *testing.T) {
	cases := map[string]string{
		"Conversa do WhatsApp com Ana Souza.txt": "Ana Souza",
		"Chat do WhatsApp com Ana Souza.zip":     "Ana Souza",
		"WhatsApp Chat with Ana Souza.txt":       "Ana Souza",
		"WhatsApp Chat - Ana Souza.zip":          "Ana Souza", // real iPhone export format
		"Chat de WhatsApp con Ana Souza.txt":     "Ana Souza",
		"Chat di WhatsApp con Ana Souza.txt":     "Ana Souza",
		"Discussion WhatsApp avec Ana Souza.txt": "Ana Souza",
		"WhatsApp-Chat mit Ana Souza.txt":        "Ana Souza",
		"WhatsApp-chat met Ana Souza.txt":        "Ana Souza",
		"Grupo da Firma.txt":                     "Grupo da Firma",
	}
	for file, expected := range cases {
		if got := ChatNameFromFile(file); got != expected {
			t.Errorf("ChatNameFromFile(%q) = %q, expected %q", file, got, expected)
		}
	}
}

func TestLooksLikePhoneNumber(t *testing.T) {
	cases := map[string]bool{
		"+55 11 91234-5678": true,
		"+1 (555) 123-4567": true,
		"5511912345678":     true,
		"+55 11 9123‑4567":  true, // non-breaking hyphen (U+2011)
		"Ana Souza":         false,
		"Grupo da Firma":    false,
		"007":               false, // too few digits to be a real phone number
		"Ana 55":            false, // letters mixed in aren't a phone
	}
	for name, expected := range cases {
		if got := LooksLikePhoneNumber(name); got != expected {
			t.Errorf("LooksLikePhoneNumber(%q) = %v, expected %v", name, got, expected)
		}
	}
}

// Compares each testdata/*.txt against its golden file; -update rewrites them.
func TestFixtures(t *testing.T) {
	entries, err := filepath.Glob("testdata/*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no fixtures found in testdata/")
	}

	for _, txtPath := range entries {
		name := filepath.Base(txtPath)
		t.Run(name, func(t *testing.T) {
			file, err := os.Open(txtPath)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()

			result, err := Parse(file, Options{})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			goldenPath := txtPath[:len(txtPath)-len(".txt")] + ".golden.json"

			if *update {
				writeGolden(t, goldenPath, result)
			}

			var expected Result
			readGolden(t, goldenPath, &expected)

			compareResult(t, result, expected)
		})
	}
}

func writeGolden(t *testing.T, path string, r Result) {
	t.Helper()
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readGolden(t *testing.T, path string, out *Result) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden (run with -update to generate it): %v", err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("invalid golden: %v", err)
	}
}

func compareResult(t *testing.T, got, expected Result) {
	t.Helper()

	gotJSON, _ := json.MarshalIndent(got, "", "  ")
	expectedJSON, _ := json.MarshalIndent(expected, "", "  ")
	if string(gotJSON) != string(expectedJSON) {
		t.Errorf("result differs from golden:\n--- got ---\n%s\n--- expected ---\n%s", gotJSON, expectedJSON)
	}
}
