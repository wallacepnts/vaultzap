package ingest

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// unit is the result of resolving an input path (loose .txt, .zip, or a subfolder with a
// .txt) to the .txt to parse and the directory to look for its attachments in.
type unit struct {
	txtPath  string
	mediaDir string
	cleanup  func() // removes the temp extraction dir, if any
	// The profile photo a chatvault archive brings along; empty for a WhatsApp export.
	avatarSource string
}

const maxSearchDepth = 2

// See extractZipSafely for why workDir must not be the system temp dir.
func resolveUnit(inputPath, workDir string) (unit, error) {
	info, err := os.Stat(inputPath)
	if err != nil {
		return unit{}, fmt.Errorf("acessar %s: %w", inputPath, err)
	}

	switch {
	case info.IsDir():
		txt, err := findTxtInDir(inputPath, maxSearchDepth)
		if err != nil {
			return unit{}, err
		}
		return unit{
			txtPath:      txt,
			mediaDir:     filepath.Dir(txt),
			cleanup:      func() {},
			avatarSource: findProfilePhoto(filepath.Dir(txt)),
		}, nil

	case strings.EqualFold(filepath.Ext(inputPath), ".zip"):
		dest, err := extractZipSafely(inputPath, workDir)
		if err != nil {
			return unit{}, fmt.Errorf("extrair %s: %w", inputPath, err)
		}
		cleanup := func() { os.RemoveAll(dest) }

		txt, err := findTxtInDir(dest, maxSearchDepth)
		if err != nil {
			cleanup()
			return unit{}, err
		}
		return unit{
			txtPath:      txt,
			mediaDir:     filepath.Dir(txt),
			cleanup:      cleanup,
			avatarSource: findProfilePhoto(filepath.Dir(txt)),
		}, nil

	default:
		return unit{txtPath: inputPath, mediaDir: filepath.Dir(inputPath), cleanup: func() {}}, nil
	}
}

func findTxtInDir(dir string, depth int) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("ler diretório %s: %w", dir, err)
	}

	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".txt") {
			return filepath.Join(dir, e.Name()), nil
		}
	}

	if depth > 0 {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			found, err := findTxtInDir(filepath.Join(dir, e.Name()), depth-1)
			if err == nil {
				return found, nil
			}
		}
	}

	return "", fmt.Errorf("nenhum .txt encontrado em %s", dir)
}

// The photo files chatvault leaves next to each conversation's _chat.txt.
var profilePhotoNames = []string{
	"profile-image.jpg", "profile-image.jpeg", "profile-image.png", "profile-image.webp",
}

func findProfilePhoto(dir string) string {
	for _, name := range profilePhotoNames {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

type InboxUnit struct {
	Name     string // relative to the inbox
	IsFolder bool
	Bytes    int64
}

// Same filters as Scanner.Scan, without touching the database.
func ListInboxUnits(inbox string) ([]InboxUnit, error) {
	entries, err := os.ReadDir(inbox)
	if err != nil {
		return nil, err
	}

	var units []InboxUnit
	for _, e := range entries {
		if isIgnorableEntry(e.Name(), e.IsDir()) {
			continue
		}
		if !e.IsDir() && !isRecognizedUnit(e.Name()) {
			continue
		}
		u := InboxUnit{Name: e.Name(), IsFolder: e.IsDir()}
		if info, err := e.Info(); err == nil && !e.IsDir() {
			u.Bytes = info.Size()
		}
		units = append(units, u)
	}
	return units, nil
}

// What the "move" policy archived, newest month first. Without it the "Reimport…" button is
// a dead end under the default policy: the file left the inbox.
//
// Names come back relative to importedDir ("2026-08/Conversa do WhatsApp com X.zip"), which
// is what applyUpdate resolves back — always inside importedDir, never a caller-chosen path.
func ListImportedUnits(importedDir string) ([]InboxUnit, error) {
	months, err := os.ReadDir(importedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var units []InboxUnit
	for i := len(months) - 1; i >= 0; i-- {
		month := months[i]
		if !month.IsDir() {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(importedDir, month.Name()))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if isIgnorableEntry(e.Name(), e.IsDir()) {
				continue
			}
			if !e.IsDir() && !isRecognizedUnit(e.Name()) {
				continue
			}
			u := InboxUnit{Name: path.Join(month.Name(), e.Name()), IsFolder: e.IsDir()}
			if info, err := e.Info(); err == nil && !e.IsDir() {
				u.Bytes = info.Size()
			}
			units = append(units, u)
		}
	}
	return units, nil
}

// ResolveImported turns a name from ListImportedUnits back into an absolute path, refusing
// anything that escapes importedDir — the name arrives from a form and is not trusted.
func ResolveImported(importedDir, name string) (string, error) {
	base, err := filepath.Abs(importedDir)
	if err != nil {
		return "", err
	}
	full := filepath.Join(base, filepath.FromSlash(name))
	if full != base && !strings.HasPrefix(full, base+string(filepath.Separator)) {
		return "", fmt.Errorf("caminho fora da pasta de importados: %q", name)
	}
	if _, err := os.Stat(full); err != nil {
		return "", err
	}
	return full, nil
}
