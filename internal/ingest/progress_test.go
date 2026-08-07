package ingest

import (
	"testing"
	"time"
)

// The bar reads Percent directly, so a zero total (unknown size) or a counter that ran
// past the total must not produce a bar wider than the box or a division by zero.
func TestSnapshotPercent(t *testing.T) {
	casos := []struct {
		nome        string
		done, total int64
		quer        int
	}{
		{"metade", 50, 100, 50},
		{"total desconhecido", 10, 0, 0},
		{"passou do total", 120, 100, 100},
		{"nada ainda", 0, 98009, 0},
		{"arredonda pra baixo", 999, 1000, 99},
	}
	for _, c := range casos {
		if got := (Snapshot{Done: c.done, Total: c.total}).Percent(); got != c.quer {
			t.Errorf("%s: Percent(%d/%d) = %d, queria %d", c.nome, c.done, c.total, got, c.quer)
		}
	}
}

func TestCurrentImport(t *testing.T) {
	if _, ok := CurrentImport(); ok {
		t.Fatal("não deveria haver import em andamento no início")
	}

	progress.start("Conversa do WhatsApp com Fulano.zip")
	progress.setPhase(PhaseExtracting, 1000)
	progress.add(250)

	s, ok := CurrentImport()
	if !ok {
		t.Fatal("deveria haver import em andamento")
	}
	if s.Phase != PhaseExtracting || s.Percent() != 25 {
		t.Errorf("fase=%s percent=%d, queria extracting/25", s.Phase, s.Percent())
	}
	if s.Elapsed > time.Minute {
		t.Errorf("elapsed absurdo: %v", s.Elapsed)
	}

	// Switching phase resets the counter: each phase counts in its own unit.
	progress.setPhase(PhaseImporting, 98009)
	if s, _ := CurrentImport(); s.Done != 0 || s.Total != 98009 {
		t.Errorf("após setPhase: done=%d total=%d, queria 0/98009", s.Done, s.Total)
	}

	progress.stopImport()
	if _, ok := CurrentImport(); ok {
		t.Error("depois de stop não deveria haver import em andamento")
	}
}

// A scan with no import in flight still has to report itself: otherwise the bar the "scan
// now" button answers with would find nothing and close before the first file is opened.
func TestScanShowsBeforeAnyImport(t *testing.T) {
	if _, ok := CurrentImport(); ok {
		t.Fatal("nada deveria estar em curso no início")
	}

	ScanStarted()
	snapshot, ok := CurrentImport()
	if !ok {
		t.Fatal("a varredura deveria aparecer mesmo sem import")
	}
	if snapshot.Importing {
		t.Error("Importing deveria ser falso durante a varredura sem arquivo")
	}

	progress.start("chat.zip")
	if snapshot, ok := CurrentImport(); !ok || !snapshot.Importing || snapshot.File != "chat.zip" {
		t.Errorf("import em curso = %+v, ok=%v", snapshot, ok)
	}

	progress.stopImport()
	if snapshot, ok := CurrentImport(); !ok || snapshot.Importing {
		t.Errorf("acabado o import, a varredura continua: %+v, ok=%v", snapshot, ok)
	}

	ScanFinished()
	if _, ok := CurrentImport(); ok {
		t.Error("acabada a varredura, nada deveria estar em curso")
	}
}
