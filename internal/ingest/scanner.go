package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wallacepnts/vaultzap/internal/config"
	"github.com/wallacepnts/vaultzap/internal/parser"
	"github.com/wallacepnts/vaultzap/internal/store"
)

const minStableAge = 5 * time.Second

const defaultPassGap = 6 * time.Second

// Never an ingestion unit, even mid-name (foo.zip.crdownload).
var partialFileSuffixes = []string{
	".part", ".crdownload", ".filepart", ".tmp", ".!qb",
}

// Single goroutine, serial.
type Scanner struct {
	store       *store.Store
	inbox       string
	mediaDir    string
	policy      config.PostImportPolicy
	importedDir string
	passGap     time.Duration
	dateOrder   parser.DateOrder
	// running serializes Scan: the startup scan and the "scan now" button are two callers
	// of the same Scanner, and overlapping them lets the loser stamp seen_files.state=
	// 'error' over the winner's 'done' — stranding the export, since 'error' with unchanged
	// (size, mtime) is never retried.
	running sync.Mutex
}

func NewScanner(s *store.Store, cfg config.Config) *Scanner {
	importedDir := cfg.ImportedDir
	if importedDir == "" {
		importedDir = filepath.Join(cfg.Inbox, ".imported")
	}
	return &Scanner{
		store:       s,
		inbox:       cfg.Inbox,
		mediaDir:    cfg.MediaDir,
		policy:      cfg.PostImportPolicy,
		importedDir: importedDir,
		passGap:     defaultPassGap,
		dateOrder:   cfg.DefaultDateOrder,
	}
}

// Takes the same lock as Scan: the "Reimport…" button used to call ImportIntoChat
// directly, so it could run alongside the startup scan — which §5.9 promises never
// happens. Progress is published like a scan's, so the bar works here too.
func (sc *Scanner) ImportInto(ctx context.Context, path, record string, chatID int64) (Report, error) {
	sc.running.Lock()
	defer sc.running.Unlock()

	progress.start(record)
	defer progress.stopImport()

	return ImportIntoChat(ctx, sc.store, path, record, sc.mediaDir, chatID, sc.dateOrder)
}

func (sc *Scanner) Scan(ctx context.Context) {
	ScanStarted()
	defer ScanFinished()

	sc.running.Lock()
	defer sc.running.Unlock()

	// A caller that arrived mid-scan would only re-walk an inbox just walked.
	if ctx.Err() != nil {
		return
	}

	sc.scanOnce(ctx)

	select {
	case <-ctx.Done():
		return
	case <-time.After(sc.passGap):
	}

	sc.scanOnce(ctx)
}

func (sc *Scanner) scanOnce(ctx context.Context) {
	entries, err := os.ReadDir(sc.inbox)
	if err != nil {
		slog.Error("ler inbox", "inbox", sc.inbox, "erro", err)
		return
	}

	for _, entry := range entries {
		if ctx.Err() != nil {
			return
		}
		name := entry.Name()
		if isIgnorableEntry(name, entry.IsDir()) || sc.isImportedDir(name) {
			continue
		}
		if !entry.IsDir() && !isRecognizedUnit(name) {
			continue
		}
		sc.processEntry(ctx, name)
	}
}

func (sc *Scanner) isImportedDir(name string) bool {
	if sc.importedDir == "" {
		return false
	}
	dest, err := filepath.Abs(sc.importedDir)
	if err != nil {
		return false
	}
	entry, err := filepath.Abs(filepath.Join(sc.inbox, name))
	if err != nil {
		return false
	}
	return entry == dest
}

func isIgnorableEntry(name string, isDir bool) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	lower := strings.ToLower(name)
	if lower == "@eadir" || lower == "lost+found" {
		return true
	}
	if !isDir {
		for _, suffix := range partialFileSuffixes {
			if strings.HasSuffix(lower, suffix) {
				return true
			}
		}
	}
	return false
}

func isRecognizedUnit(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".txt" || ext == ".zip"
}

// pending -> stable -> done/error. Entries already resolved with unchanged (size, mtime)
// are skipped.
func (sc *Scanner) processEntry(ctx context.Context, relativeName string) {
	absPath := filepath.Join(sc.inbox, relativeName)
	info, err := os.Stat(absPath)
	if err != nil {
		slog.Warn("stat na inbox", "arquivo", relativeName, "erro", err)
		return
	}

	size, modTime := unitStat(absPath, info)
	mtime := modTime.Format("2006-01-02 15:04:05")
	now := time.Now().Format("2006-01-02 15:04:05")

	previous, existed, err := sc.store.GetSeenFile(ctx, relativeName)
	if err != nil {
		slog.Error("consultar seen_files", "arquivo", relativeName, "erro", err)
		return
	}

	unchanged := existed && previous.Size == size && previous.Mtime == mtime
	resolved := existed && (previous.State == store.StateDone || previous.State == store.StateError || previous.State == store.StateIgnored)

	if unchanged && resolved {
		// Reapplied only to files already 'done': re-applying "delete" would retroactively
		// remove a file that had been spared for having warnings.
		if previous.State == store.StateDone && sc.policy == config.PolicyMove {
			sc.moveToImported(relativeName, absPath)
		}
		_ = sc.store.SaveSeenFile(ctx, store.SeenFile{
			Path: relativeName, Size: size, Mtime: mtime,
			SHA256: previous.SHA256, State: previous.State, LastSeen: now,
		})
		return
	}

	newState := store.StatePending
	if unchanged && time.Since(modTime) >= minStableAge {
		newState = store.StateStable
	}

	if err := sc.store.SaveSeenFile(ctx, store.SeenFile{
		Path: relativeName, Size: size, Mtime: mtime,
		SHA256: previous.SHA256, State: newState, LastSeen: now,
	}); err != nil {
		slog.Error("salvar seen_files", "arquivo", relativeName, "erro", err)
		return
	}

	if newState != store.StateStable {
		return
	}
	sc.doImport(ctx, relativeName, absPath, size, mtime)
}

// maxUnitDepth bounds how deep unitStat walks: a subfolder is a unit, not infinite
// recursion.
const maxUnitDepth = 2

// The (size, mtime) pair stability detection compares across passes. For a folder it is
// the sum of the sizes and the newest mtime INSIDE, not the folder's own: a directory's
// mtime only changes when an entry is added or removed, so a folder whose last file is
// still being written looks perfectly still, and would be declared stable with half-copied
// media. The folder's own values stay as the floor, so an empty folder still ages.
func unitStat(path string, info os.FileInfo) (int64, time.Time) {
	if !info.IsDir() {
		return info.Size(), info.ModTime()
	}

	total := int64(0)
	newest := info.ModTime()

	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, entry := range entries {
			entryInfo, err := entry.Info()
			if err != nil {
				continue
			}
			if entry.IsDir() {
				if entryInfo.ModTime().After(newest) {
					newest = entryInfo.ModTime()
				}
				if depth < maxUnitDepth {
					walk(filepath.Join(dir, entry.Name()), depth+1)
				}
				continue
			}
			total += entryInfo.Size()
			if entryInfo.ModTime().After(newest) {
				newest = entryInfo.ModTime()
			}
		}
	}
	walk(path, 1)

	return total, newest
}

// Calls the same code as `vaultzap ingest`. A malformed file never takes down the scan.
func (sc *Scanner) doImport(ctx context.Context, relativeName, absPath string, size int64, mtime string) {
	// The /imports page reads this while the import runs; hashForSeenFiles below is part
	// of the work, so the tracker only stops after it.
	progress.start(relativeName)
	defer progress.stopImport()

	report, err := ImportFile(ctx, sc.store, absPath, relativeName, sc.mediaDir, sc.dateOrder)
	if err == nil && report.Added > 0 {
		markImported()
	}
	now := time.Now().Format("2006-01-02 15:04:05")

	state := store.StateDone
	if err != nil {
		slog.Error("importar da inbox", "arquivo", relativeName, "erro", err)
		state = store.StateError
	}

	if errSave := sc.store.SaveSeenFile(ctx, store.SeenFile{
		Path: relativeName, Size: size, Mtime: mtime,
		SHA256: hashForSeenFiles(absPath), State: state, LastSeen: now,
	}); errSave != nil {
		slog.Error("salvar seen_files após import", "arquivo", relativeName, "erro", errSave)
	}

	if err == nil {
		sc.applyPolicy(relativeName, absPath, report)
	}
}

// Failures are only logged.
func (sc *Scanner) moveToImported(relativeName, absPath string) {
	dest := filepath.Join(sc.importedDir, time.Now().Format("2006-01"))
	if err := os.MkdirAll(dest, 0o755); err != nil {
		slog.Error("criar diretório de importados", "destino", dest, "erro", err)
		return
	}

	target, err := freeDestination(dest, filepath.Base(absPath))
	if err != nil {
		slog.Error("achar nome livre em importados", "arquivo", relativeName, "erro", err)
		return
	}

	if err := os.Rename(absPath, target); err != nil {
		// EXDEV: the destination is on another filesystem, which an archive disk makes a
		// supported case. Rename can't cross it; without the copy fallback the file stays in
		// the inbox and every scan retries the same failing rename forever.
		if !errors.Is(err, syscall.EXDEV) {
			slog.Error("mover da inbox", "arquivo", relativeName, "erro", err)
			return
		}
		if err := copyAcrossFilesystems(absPath, target); err != nil {
			slog.Error("copiar da inbox pra outro sistema de arquivos", "arquivo", relativeName, "erro", err)
			return
		}
		if err := os.RemoveAll(absPath); err != nil {
			slog.Error("remover da inbox após copiar", "arquivo", relativeName, "erro", err)
			return
		}
	}
	slog.Info("arquivo movido da inbox", "arquivo", relativeName, "destino", target)
}

const maxDestinationAttempts = 1000

// WhatsApp always reuses the same export file name, so renaming straight onto the
// destination would silently overwrite an export archived earlier.
func freeDestination(dir, name string) (string, error) {
	candidate := filepath.Join(dir, name)
	if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
		return candidate, nil
	}

	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 2; i <= maxDestinationAttempts; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("sem nome livre para %q em %s", name, dir)
}

// Copies a file or a whole ingestion folder; the EXDEV fallback of moveToImported.
func copyAcrossFilesystems(src, dest string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dest)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		if err := copyAcrossFilesystems(filepath.Join(src, entry.Name()), filepath.Join(dest, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// VAULTZAP_AFTER_IMPORT, except that "delete" is skipped when the import had warnings.
func (sc *Scanner) applyPolicy(relativeName, absPath string, report Report) {
	switch sc.policy {
	case config.PolicyKeep:
		return

	case config.PolicyMove:
		sc.moveToImported(relativeName, absPath)

	case config.PolicyDelete:
		if report.Warnings > 0 {
			return
		}
		if err := os.RemoveAll(absPath); err != nil {
			slog.Error("excluir da inbox", "arquivo", relativeName, "erro", err)
		}
	}
}

// Fails silently, returning nil.
func hashForSeenFiles(path string) *string {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil
	}
	sum, err := hashFileHex(path)
	if err != nil {
		return nil
	}
	return &sum
}
