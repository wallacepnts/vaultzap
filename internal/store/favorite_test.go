package store

import (
	"context"
	"testing"
)

func TestToggleMessageFavorite(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c1 := createTestChat(t, s, "Ana", false)
	insertTestMessage(t, s, c1, "2026-07-26 10:00:00", 1, ptr("Ana"), "oi", "text")
	insertTestMessage(t, s, c1, "2026-07-26 10:01:00", 2, ptr("Ana"), "tudo bem?", "text")
	insertTestMessage(t, s, c1, "2026-07-26 10:02:00", 3, nil, "Ana entrou no grupo", "system")

	c2 := createTestChat(t, s, "Bruno", false)
	insertTestMessage(t, s, c2, "2026-07-26 11:00:00", 1, ptr("Bruno"), "oi de novo", "text")

	msgsC1, _, err := s.LastMessagePage(ctx, c1, 0)
	if err != nil {
		t.Fatalf("LastMessagePage c1: %v", err)
	}
	msgsC2, _, err := s.LastMessagePage(ctx, c2, 0)
	if err != nil {
		t.Fatalf("LastMessagePage c2: %v", err)
	}
	oi, tudoBem, warning := msgsC1[0], msgsC1[1], msgsC1[2]
	deOutroChat := msgsC2[0]

	favorite, pinned, sentAt, err := s.ToggleMessageFavorite(ctx, c1, oi.ID)
	if err != nil || !favorite || pinned || sentAt != oi.SentAt {
		t.Fatalf("favoritar 'oi': favorite=%v pinned=%v sentAt=%q err=%v", favorite, pinned, sentAt, err)
	}
	favorite, pinned, _, err = s.ToggleMessageFavorite(ctx, c1, oi.ID)
	if err != nil || favorite || pinned {
		t.Fatalf("desfavoritar 'oi': favorite=%v pinned=%v err=%v", favorite, pinned, err)
	}

	if _, _, _, err := s.ToggleMessageFavorite(ctx, c1, tudoBem.ID); err != nil {
		t.Fatalf("favoritar 'tudo bem?': %v", err)
	}

	if _, _, _, err := s.ToggleMessageFavorite(ctx, c1, warning.ID); err == nil {
		t.Error("favoritar mensagem de sistema deveria falhar, não falhou")
	}

	if _, _, _, err := s.ToggleMessageFavorite(ctx, c1, deOutroChat.ID); err == nil {
		t.Error("favoritar mensagem de outro chat deveria falhar, não falhou")
	}

	favoritas, err := s.ListFavoriteMessages(ctx, c1)
	if err != nil {
		t.Fatalf("ListFavoriteMessages: %v", err)
	}
	if len(favoritas) != 1 || favoritas[0].ID != tudoBem.ID {
		t.Fatalf("favoritas de c1 = %+v, esperado só 'tudo bem?' (id=%d)", favoritas, tudoBem.ID)
	}
	if !favoritas[0].Favorite {
		t.Error("Message.Favorite deveria vir true na listagem")
	}

	favoritasC2, err := s.ListFavoriteMessages(ctx, c2)
	if err != nil {
		t.Fatalf("ListFavoriteMessages c2: %v", err)
	}
	if len(favoritasC2) != 0 {
		t.Errorf("favoritas de c2 = %+v, esperado nenhuma (escopo por chat)", favoritasC2)
	}
}
