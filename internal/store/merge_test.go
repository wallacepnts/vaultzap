package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestMergeChats(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	ana := "Ana"

	dest := createTestChat(t, s, "Ana Souza", false)
	source := createTestChat(t, s, "Ana Souza 2025", false)

	insertTestMessage(t, s, dest, "2026-07-20 10:00:00", 1, &ana, "só no destino", "text")
	insertTestMessage(t, s, source, "2026-07-21 10:00:00", 1, &ana, "só na origem", "text")
	for _, id := range []int64{dest, source} {
		hash := "compartilhado"
		if _, err := s.DB().Exec(
			`INSERT INTO messages (chat_id, sent_at, seq, sender, body, kind, hash) VALUES (?,?,?,?,?,?,?)`,
			id, "2026-07-19 09:00:00", 99, ana, "nos dois", "text", hash); err != nil {
			t.Fatalf("inserir mensagem compartilhada: %v", err)
		}
	}
	insertTestAttachment(t, s, dest, "foto.jpg", "image", "sha-repetido", 10)
	sourceAttachment := insertTestAttachment(t, s, source, "foto.jpg", "image", "sha-repetido", 10)
	insertTestAttachment(t, s, source, "outra.jpg", "image", "sha-exclusivo", 20)
	if _, err := s.DB().Exec(`UPDATE messages SET attachment_id = ? WHERE chat_id = ? AND seq = 1`, sourceAttachment, source); err != nil {
		t.Fatal(err)
	}

	if err := s.SetOwner(ctx, source, "Wallace"); err != nil {
		t.Fatal(err)
	}

	report, err := s.MergeChats(ctx, dest, source)
	if err != nil {
		t.Fatalf("FundirChats: %v", err)
	}
	if report.MessagesMoved != 1 || report.MessagesDuplicated != 1 {
		t.Errorf("relatório = %d movidas / %d duplicadas, esperado 1/1", report.MessagesMoved, report.MessagesDuplicated)
	}
	if report.AttachmentsMoved != 1 || report.AttachmentsDuplicated != 1 {
		t.Errorf("anexos = %d movidos / %d duplicados, esperado 1/1", report.AttachmentsMoved, report.AttachmentsDuplicated)
	}

	if _, err := s.GetChat(ctx, source); err == nil {
		t.Error("a conversa de origem deveria ter sido apagada")
	}

	final, err := s.GetChat(ctx, dest)
	if err != nil {
		t.Fatalf("BuscarChat destino: %v", err)
	}
	if final.MessageCount != 3 {
		t.Errorf("total = %d, esperado 3 (1 do destino + 1 da origem + 1 compartilhada)", final.MessageCount)
	}
	if final.Owner == nil || *final.Owner != "Wallace" {
		t.Error("o destino deveria herdar o dono que só a origem tinha")
	}

	var orphanMessages int
	s.DB().QueryRow(`SELECT COUNT(*) FROM messages m WHERE m.attachment_id IS NOT NULL
		AND NOT EXISTS (SELECT 1 FROM attachments a WHERE a.id = m.attachment_id)`).Scan(&orphanMessages)
	if orphanMessages != 0 {
		t.Errorf("%d mensagens apontam pra anexo inexistente", orphanMessages)
	}
}

// The merge people actually do: the media-less .txt and the .zip of the same chat, where
// the surviving message must end up with the attachment.
func TestMergeChats_rescuesDuplicateMessageMedia(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	ana := "Ana"

	dest := createTestChat(t, s, "Ana Souza", false)       // from the .txt, no media
	source := createTestChat(t, s, "Ana Souza zip", false) // from the .zip, with media

	const hash = "mensagem-com-foto"
	for _, id := range []int64{dest, source} {
		kind := "media_omitted"
		if id == source {
			kind = "media"
		}
		if _, err := s.DB().Exec(
			`INSERT INTO messages (chat_id, sent_at, seq, sender, body, kind, hash) VALUES (?,?,?,?,?,?,?)`,
			id, "2026-07-20 10:00:00", 1, ana, "IMG-20260720-WA0001.jpg", kind, hash); err != nil {
			t.Fatalf("inserir mensagem: %v", err)
		}
	}
	attachment := insertTestAttachment(t, s, source, "IMG-20260720-WA0001.jpg", "image", "sha-da-foto", 100)
	if _, err := s.DB().Exec(`UPDATE messages SET attachment_id = ? WHERE chat_id = ?`, attachment, source); err != nil {
		t.Fatal(err)
	}

	if _, err := s.MergeChats(ctx, dest, source); err != nil {
		t.Fatalf("MergeChats: %v", err)
	}

	var attachmentID *int64
	var kind string
	err := s.DB().QueryRow(`SELECT attachment_id, kind FROM messages WHERE chat_id = ?`, dest).
		Scan(&attachmentID, &kind)
	if err != nil {
		t.Fatalf("consultar mensagem do destino: %v", err)
	}
	if attachmentID == nil {
		t.Fatal("a mensagem sobrevivente ficou sem anexo: a mídia se perdeu na mesclagem")
	}
	if *attachmentID != attachment {
		t.Errorf("attachment_id = %d, esperado %d", *attachmentID, attachment)
	}
	if kind != "media" {
		t.Errorf("kind = %q, esperado media (era media_omitted e agora tem arquivo)", kind)
	}

	var orphanAttachments int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM attachments a
		WHERE NOT EXISTS (SELECT 1 FROM messages m WHERE m.attachment_id = a.id)`).Scan(&orphanAttachments); err != nil {
		t.Fatal(err)
	}
	if orphanAttachments != 0 {
		t.Errorf("%d anexos sem nenhuma mensagem apontando pra eles", orphanAttachments)
	}
}

// Both sides can be at the pin limit, and nothing else repairs a merged chat over it.
func TestMergeChats_reappliesPinLimit(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	dest := createTestChat(t, s, "Ana", false)
	source := createTestChat(t, s, "Ana antiga", false)

	fixarTodas := func(chatID int64, prefixo string) {
		for i := 1; i <= MaxPinnedMessages; i++ {
			insertTestMessage(t, s, chatID, fmt.Sprintf("2026-07-1%d 10:00:00", i), i,
				ptr("Ana"), fmt.Sprintf("%s%d", prefixo, i), "text")
		}
		msgs, _, err := s.LastMessagePage(ctx, chatID, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range msgs {
			if _, _, _, err := s.TogglePinnedMessage(ctx, chatID, m.ID); err != nil {
				t.Fatalf("fixar %s: %v", m.Body, err)
			}
		}
	}
	fixarTodas(dest, "destino")
	fixarTodas(source, "origem")

	if _, err := s.MergeChats(ctx, dest, source); err != nil {
		t.Fatalf("MergeChats: %v", err)
	}

	pinnedList, err := s.ListPinnedMessages(ctx, dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(pinnedList) != MaxPinnedMessages {
		bodies := make([]string, 0, len(pinnedList))
		for _, m := range pinnedList {
			bodies = append(bodies, m.Body)
		}
		t.Errorf("fixadas depois da mesclagem = %d %v, esperado %d", len(pinnedList), bodies, MaxPinnedMessages)
	}
}

// SQLite's CURRENT_TIMESTAMP is UTC while sent_at is the export's local time; mixing the
// two scales misorders the pinned strip.
func TestTogglePinnedMessage_pinnedAtIsLocalTime(t *testing.T) {
	if _, offset := time.Now().Zone(); offset == 0 {
		t.Skip("fuso local igual a UTC: o teste não distingue as duas escalas")
	}

	s := openTestStore(t)
	ctx := context.Background()
	c := createTestChat(t, s, "Ana", false)
	insertTestMessage(t, s, c, "2026-07-20 10:00:00", 1, ptr("Ana"), "msg", "text")

	msgs, _, err := s.LastMessagePage(ctx, c, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := s.TogglePinnedMessage(ctx, c, msgs[0].ID); err != nil {
		t.Fatal(err)
	}

	var pinnedAt string
	if err := s.DB().QueryRow(`SELECT pinned_at FROM messages WHERE id = ?`, msgs[0].ID).Scan(&pinnedAt); err != nil {
		t.Fatal(err)
	}
	stored, err := time.ParseInLocation("2006-01-02 15:04:05", pinnedAt, time.Local)
	if err != nil {
		t.Fatalf("pinned_at = %q, não parseia como timestamp local: %v", pinnedAt, err)
	}
	if diff := time.Since(stored); diff > time.Minute || diff < -time.Minute {
		t.Errorf("pinned_at = %q, %v longe do agora local (parece estar em UTC)", pinnedAt, diff)
	}
}
