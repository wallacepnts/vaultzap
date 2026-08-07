package store

import (
	"context"
	"fmt"
	"testing"
)

func TestTogglePinnedMessage(t *testing.T) {
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

	favorite, pinned, _, err := s.TogglePinnedMessage(ctx, c1, oi.ID)
	if err != nil || !pinned || favorite {
		t.Fatalf("fixar 'oi': favorite=%v pinned=%v err=%v", favorite, pinned, err)
	}
	favorite, pinned, _, err = s.TogglePinnedMessage(ctx, c1, oi.ID)
	if err != nil || pinned || favorite {
		t.Fatalf("desafixar 'oi': favorite=%v pinned=%v err=%v", favorite, pinned, err)
	}

	if _, _, _, err := s.TogglePinnedMessage(ctx, c1, tudoBem.ID); err != nil {
		t.Fatalf("fixar 'tudo bem?': %v", err)
	}

	if _, _, _, err := s.TogglePinnedMessage(ctx, c1, warning.ID); err == nil {
		t.Error("fixar mensagem de sistema deveria falhar, não falhou")
	}
	if _, _, _, err := s.TogglePinnedMessage(ctx, c1, deOutroChat.ID); err == nil {
		t.Error("fixar mensagem de outro chat deveria falhar, não falhou")
	}

	pinnedList, err := s.ListPinnedMessages(ctx, c1)
	if err != nil {
		t.Fatalf("ListPinnedMessages: %v", err)
	}
	if len(pinnedList) != 1 || pinnedList[0].ID != tudoBem.ID {
		t.Fatalf("fixadas de c1 = %+v, esperado só 'tudo bem?' (id=%d)", pinnedList, tudoBem.ID)
	}
	if !pinnedList[0].Pinned {
		t.Error("Message.Pinned deveria vir true na listagem")
	}

	pinnedListC2, err := s.ListPinnedMessages(ctx, c2)
	if err != nil {
		t.Fatalf("ListPinnedMessages c2: %v", err)
	}
	if len(pinnedListC2) != 0 {
		t.Errorf("fixadas de c2 = %+v, esperado nenhuma (escopo por chat)", pinnedListC2)
	}

	if favorite, _, _, err := s.ToggleMessageFavorite(ctx, c1, tudoBem.ID); err != nil || !favorite {
		t.Fatalf("favoritar 'tudo bem?' (já fixada): favorite=%v err=%v", favorite, err)
	}
	favoritesAndPins, err := s.ListFavoriteMessages(ctx, c1)
	if err != nil {
		t.Fatalf("ListFavoriteMessages: %v", err)
	}
	if len(favoritesAndPins) != 1 || !favoritesAndPins[0].Pinned || !favoritesAndPins[0].Favorite {
		t.Fatalf("'tudo bem?' deveria estar favorita e fixada ao mesmo tempo: %+v", favoritesAndPins)
	}
}

// The 5th pin evicts the one pinned first — not itself, and not the oldest message.
func TestTogglePinnedMessage_limit(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c := createTestChat(t, s, "Ana", false)
	// Inserted newest-first on purpose, so sent_at order is the reverse of pinning order.
	for i := 5; i >= 1; i-- {
		insertTestMessage(t, s, c, fmt.Sprintf("2026-07-2%d 10:00:00", i), i, ptr("Ana"), fmt.Sprintf("msg%d", i), "text")
	}
	msgs, _, err := s.LastMessagePage(ctx, c, 0)
	if err != nil {
		t.Fatalf("LastMessagePage: %v", err)
	}
	byBody := map[string]int64{}
	for _, m := range msgs {
		byBody[m.Body] = m.ID
	}

	for _, body := range []string{"msg1", "msg2", "msg3", "msg4"} {
		if _, _, _, err := s.TogglePinnedMessage(ctx, c, byBody[body]); err != nil {
			t.Fatalf("fixar %s: %v", body, err)
		}
		if _, err := s.DB().Exec(`UPDATE messages SET pinned_at = ? WHERE id = ?`,
			fmt.Sprintf("2020-01-01 10:0%s:00", body[3:]), byBody[body]); err != nil {
			t.Fatalf("ajustar pinned_at de %s: %v", body, err)
		}
	}
	pinnedList, err := s.ListPinnedMessages(ctx, c)
	if err != nil {
		t.Fatalf("ListPinnedMessages: %v", err)
	}
	if len(pinnedList) != MaxPinnedMessages {
		t.Fatalf("com 4 fixadas, len = %d, esperado %d", len(pinnedList), MaxPinnedMessages)
	}
	if pinnedList[0].Body != "msg4" || pinnedList[3].Body != "msg1" {
		t.Errorf("ordem = %s..%s, esperado msg4 primeiro e msg1 por último (ordem de fixação)",
			pinnedList[0].Body, pinnedList[3].Body)
	}

	if _, _, _, err := s.TogglePinnedMessage(ctx, c, byBody["msg5"]); err != nil {
		t.Fatalf("fixar msg5: %v", err)
	}
	pinnedList, err = s.ListPinnedMessages(ctx, c)
	if err != nil {
		t.Fatalf("ListPinnedMessages: %v", err)
	}
	if len(pinnedList) != MaxPinnedMessages {
		t.Fatalf("depois da 5ª, len = %d, esperado continuar %d", len(pinnedList), MaxPinnedMessages)
	}
	for _, m := range pinnedList {
		if m.Body == "msg1" {
			t.Error("msg1 (fixada há mais tempo) deveria ter saído, continua fixada")
		}
	}
	if pinnedList[0].Body != "msg5" {
		t.Errorf("primeira da faixa = %s, esperado msg5 (fixada agora)", pinnedList[0].Body)
	}
}
