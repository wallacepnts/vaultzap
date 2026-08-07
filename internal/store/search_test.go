package store

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestSearchMessages_findsByText(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	chatID := createTestChat(t, s, "Amigos", false)

	insertTestMessage(t, s, chatID, "2026-07-26 10:00:00", 1, ptr("Ana"), "vamos pro cinema hoje?", "text")
	insertTestMessage(t, s, chatID, "2026-07-26 10:01:00", 2, ptr("Bruno"), "beleza, que horas?", "text")
	insertTestMessage(t, s, chatID, "2026-07-26 10:02:00", 3, ptr("Ana"), "as 19h no shopping", "text")

	results, err := s.SearchMessages(ctx, chatID, "cinema")
	if err != nil {
		t.Fatalf("BuscarMensagens: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("esperava 1 resultado, veio %d", len(results))
	}
	if results[0].Message.Body != "vamos pro cinema hoje?" {
		t.Errorf("mensagem encontrada = %q, inesperada", results[0].Message.Body)
	}
	if !strings.Contains(results[0].Snippet, snippetMarkStart+"cinema"+snippetMarkEnd) {
		t.Errorf("trecho = %q, esperava marcação em volta de 'cinema'", results[0].Snippet)
	}
}

func TestSearchMessages_ignoresAccents(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	chatID := createTestChat(t, s, "Amigos", false)
	insertTestMessage(t, s, chatID, "2026-07-26 10:00:00", 1, ptr("Ana"), "você já chegou?", "text")

	results, err := s.SearchMessages(ctx, chatID, "voce")
	if err != nil {
		t.Fatalf("BuscarMensagens: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("busca sem acento deveria achar 'você', veio %d resultados", len(results))
	}
}

func TestSearchMessages_doesNotLeakAnotherChat(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	chat1 := createTestChat(t, s, "Chat 1", false)
	chat2 := createTestChat(t, s, "Chat 2", false)
	insertTestMessage(t, s, chat1, "2026-07-26 10:00:00", 1, ptr("A"), "senha do wifi é 12345", "text")
	insertTestMessage(t, s, chat2, "2026-07-26 10:00:00", 1, ptr("B"), "senha do wifi é outra", "text")

	results, err := s.SearchMessages(ctx, chat1, "senha")
	if err != nil {
		t.Fatalf("BuscarMensagens: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("esperava 1 resultado restrito ao chat1, veio %d", len(results))
	}
}

func TestMessagesAround(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	chatID := createTestChat(t, s, "Janela", false)

	const total = 60
	var idAlvo int64
	for i := 1; i <= total; i++ {
		sentAt := fmt.Sprintf("2026-07-26 %02d:%02d:00", i/60, i%60)
		insertTestMessage(t, s, chatID, sentAt, i, ptr("Vitor"), fmt.Sprintf("mensagem %d", i), "text")
		if i == 30 {
			if err := s.DB().QueryRowContext(ctx, `SELECT id FROM messages WHERE chat_id = ? AND seq = ?`, chatID, i).Scan(&idAlvo); err != nil {
				t.Fatalf("buscar id da mensagem alvo: %v", err)
			}
		}
	}

	janela, hasMore, err := s.MessagesAround(ctx, chatID, idAlvo)
	if err != nil {
		t.Fatalf("MensagensAoRedor: %v", err)
	}
	if len(janela) == 0 {
		t.Fatal("janela vazia")
	}
	if janela[0].SentAt > janela[len(janela)-1].SentAt {
		t.Error("janela fora de ordem cronológica")
	}

	achouAlvo := false
	for _, m := range janela {
		if m.ID == idAlvo {
			achouAlvo = true
		}
	}
	if !achouAlvo {
		t.Error("mensagem alvo não está na janela retornada")
	}
	if !hasMore {
		t.Error("esperava temMais=true (existem mensagens antes da janela)")
	}
}

func TestListChats_globalSearchByMessageBody(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	chatID := createTestChat(t, s, "Time do Trabalho", false)
	insertTestMessage(t, s, chatID, "2026-07-26 10:00:00", 1, ptr("Ana"), "reunião marcada pra sexta-feira", "text")

	chats, err := s.ListChats(ctx, ChatFilter{Search: "sexta-feira"})
	if err != nil {
		t.Fatalf("ListarChats: %v", err)
	}
	if len(chats) != 1 || chats[0].Name != "Time do Trabalho" {
		t.Errorf("busca global deveria achar o chat pelo conteúdo da mensagem, veio %+v", chats)
	}
}
