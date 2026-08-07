package ingest

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// A zip bomb is a file that expands enormously; a WhatsApp export is mostly media,
// which is already compressed and comes out near 1:1. So the guard is the expansion
// RATIO, not an absolute size — a real 10-year group export runs to gigabytes and
// thousands of entries, and the old absolute caps (2 GiB, 10k entries) refused it.
// The absolute limits stay as a backstop, high enough not to hit a real archive.
const (
	maxZipEntries   = 100_000
	maxUncompressed = 64 << 30 // backstop; a real export is orders of magnitude below
	maxEntrySize    = 4 << 30

	// Text compresses ~10x, so this only trips on something engineered to expand.
	maxExpansionRatio = 100
	// Below this, ratio means nothing: a 20-byte file can "expand" 50x and be harmless.
	ratioFloor = 8 << 20
)

// extractZipSafely extracts zipPath into a fresh temp directory inside workDir, rejecting
// entries that escape it or blow past the limits above. The caller removes the directory.
//
// workDir is MEDIA_DIR, not the system temp dir, and that is the whole point: the
// container runs with --read-only and a --tmpfs /tmp, so the system temp dir is RAM and
// a routine 800 MB export would sit in memory for the length of the import. MEDIA_DIR is
// writable by definition, real disk in every supported deployment, and being the same
// filesystem as the destination keeps the attachment copy from crossing devices.
// Empty workDir falls back to the system temp dir, which is what the tests use.
func extractZipSafely(zipPath, workDir string) (dest string, err error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("abrir zip: %w", err)
	}
	defer reader.Close()

	if len(reader.File) > maxZipEntries {
		return "", fmt.Errorf("zip com %d entradas, acima do limite de %d", len(reader.File), maxZipEntries)
	}

	// Dot prefix so a half-extracted unit never looks like content to anything walking the
	// media tree.
	dest, err = os.MkdirTemp(workDir, ".vaultzap-zip-*")
	if err != nil {
		return "", fmt.Errorf("criar diretório temporário: %w", err)
	}

	if err := extractEntries(reader.File, dest); err != nil {
		os.RemoveAll(dest)
		return "", err
	}

	return dest, nil
}

func extractEntries(files []*zip.File, dest string) error {
	var totalUncompressed, totalCompressed uint64

	var declared int64
	for _, f := range files {
		declared += int64(f.UncompressedSize64)
	}
	progress.setPhase(PhaseExtracting, declared)

	for _, f := range files {
		destPath, err := safePath(dest, f.Name)
		if err != nil {
			return err
		}

		if f.UncompressedSize64 > maxEntrySize {
			return fmt.Errorf("entrada %q maior que o limite por arquivo (%d bytes)", f.Name, uint64(maxEntrySize))
		}
		if expandsTooMuch(f.UncompressedSize64, f.CompressedSize64) {
			return fmt.Errorf("entrada %q expande %dx (limite %dx): parece zip bomb",
				f.Name, f.UncompressedSize64/max(f.CompressedSize64, 1), uint64(maxExpansionRatio))
		}
		totalUncompressed += f.UncompressedSize64
		totalCompressed += f.CompressedSize64
		if totalUncompressed > maxUncompressed {
			return fmt.Errorf("zip descomprime para mais que o limite total (%d bytes)", uint64(maxUncompressed))
		}
		if expandsTooMuch(totalUncompressed, totalCompressed) {
			return fmt.Errorf("zip expande %dx no total (limite %dx): parece zip bomb",
				totalUncompressed/max(totalCompressed, 1), uint64(maxExpansionRatio))
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		if err := extractFile(f, destPath); err != nil {
			return err
		}
		progress.add(int64(f.UncompressedSize64))
	}

	return nil
}

// Sizes come from the zip header, which a crafted file can lie about — the second check in
// extractFile is what enforces the limit on the bytes actually written.
func expandsTooMuch(uncompressed, compressed uint64) bool {
	return uncompressed > ratioFloor && uncompressed > compressed*maxExpansionRatio
}

// Rejects an absolute path or any attempt to escape base via "..".
func safePath(base, name string) (string, error) {
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("entrada com caminho absoluto rejeitada: %q", name)
	}

	clean := filepath.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("entrada tenta escapar do destino: %q", name)
	}

	baseAbs := filepath.Clean(base)
	dest := filepath.Join(baseAbs, clean)
	if dest != baseAbs && !strings.HasPrefix(dest, baseAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("entrada tenta escapar do destino: %q", name)
	}
	return dest, nil
}

// Second size check, defense in depth: the zip header can lie about UncompressedSize64.
func extractFile(f *zip.File, dest string) error {
	src, err := f.Open()
	if err != nil {
		return fmt.Errorf("abrir entrada %q: %w", f.Name, err)
	}
	defer src.Close()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("criar %q: %w", dest, err)
	}
	defer out.Close()

	copied, err := io.CopyN(out, src, maxEntrySize)
	if err != nil && err != io.EOF {
		return fmt.Errorf("extrair %q: %w", f.Name, err)
	}
	if copied == maxEntrySize {
		var extra [1]byte
		if n, _ := src.Read(extra[:]); n > 0 {
			return fmt.Errorf("entrada %q excede o limite por arquivo", f.Name)
		}
	}
	return nil
}
