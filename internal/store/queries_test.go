package store

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func createTestChat(t *testing.T, s *Store, name string, ehGrupo bool) int64 {
	t.Helper()
	res, err := s.DB().Exec(`INSERT INTO chats (name, is_group, source, created_at) VALUES (?, ?, 'android', '2026-07-26 00:00:00')`,
		name, ehGrupo)
	if err != nil {
		t.Fatalf("criar chat: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func insertTestMessage(t *testing.T, s *Store, chatID int64, sentAt string, seq int, sender *string, body, kind string) {
	t.Helper()
	hash := fmt.Sprintf("%d-%s-%d", chatID, sentAt, seq)
	_, err := s.DB().Exec(`INSERT INTO messages (chat_id, sent_at, seq, sender, body, kind, hash) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		chatID, sentAt, seq, sender, body, kind, hash)
	if err != nil {
		t.Fatalf("inserir mensagem: %v", err)
	}
	_, err = s.DB().Exec(`UPDATE chats SET
		first_message_at = (SELECT MIN(sent_at) FROM messages WHERE chat_id = ?),
		last_message_at  = (SELECT MAX(sent_at) FROM messages WHERE chat_id = ?),
		message_count    = (SELECT COUNT(*) FROM messages WHERE chat_id = ?)
		WHERE id = ?`, chatID, chatID, chatID, chatID)
	if err != nil {
		t.Fatalf("atualizar resumo: %v", err)
	}
}

func ptr(s string) *string { return &s }

func TestListChats_orderAndSearch(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c1 := createTestChat(t, s, "Ana", false)
	insertTestMessage(t, s, c1, "2026-07-26 10:00:00", 1, ptr("Ana"), "oi", "text")

	c2 := createTestChat(t, s, "Bruno", false)
	insertTestMessage(t, s, c2, "2026-07-26 12:00:00", 1, ptr("Bruno"), "e aí", "text")

	chats, err := s.ListChats(ctx, ChatFilter{})
	if err != nil {
		t.Fatalf("ListarChats: %v", err)
	}
	if len(chats) != 2 {
		t.Fatalf("esperava 2 chats, veio %d", len(chats))
	}
	if chats[0].Name != "Bruno" {
		t.Errorf("primeiro chat = %q, esperado Bruno (mensagem mais recente)", chats[0].Name)
	}
	if chats[0].PreviewBody != "e aí" {
		t.Errorf("prévia do Bruno = %q, esperado %q", chats[0].PreviewBody, "e aí")
	}

	filtrados, err := s.ListChats(ctx, ChatFilter{Search: "an"})
	if err != nil {
		t.Fatalf("ListarChats com busca: %v", err)
	}
	if len(filtrados) != 1 || filtrados[0].Name != "Ana" {
		t.Errorf("busca 'an' deveria retornar só Ana, veio %+v", filtrados)
	}
}

func TestGetChat(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	id := createTestChat(t, s, "Grupo Teste", true)

	c, err := s.GetChat(ctx, id)
	if err != nil {
		t.Fatalf("BuscarChat: %v", err)
	}
	if c.Name != "Grupo Teste" || !c.IsGroup {
		t.Errorf("chat = %+v, inesperado", c)
	}

	if _, err := s.GetChat(ctx, 9999); err == nil {
		t.Error("esperava erro para chat inexistente")
	}
}

func TestRenameChat(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	id := createTestChat(t, s, "Ana", false)
	createTestChat(t, s, "Bruno", false)

	if err := s.RenameChat(ctx, id, "Ana Souza"); err != nil {
		t.Fatalf("RenomearChat: %v", err)
	}
	c, err := s.GetChat(ctx, id)
	if err != nil {
		t.Fatalf("BuscarChat: %v", err)
	}
	if c.Name != "Ana Souza" {
		t.Errorf("nome = %q, esperado Ana Souza", c.Name)
	}

	if err := s.RenameChat(ctx, id, "Bruno"); err == nil {
		t.Error("esperava erro de UNIQUE constraint ao renomear para nome já existente")
	}
}

func TestSetAvatar(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	id := createTestChat(t, s, "Ana", false)

	if err := s.SetAvatar(ctx, id, "avatars/1.jpg"); err != nil {
		t.Fatalf("DefinirAvatar: %v", err)
	}
	c, err := s.GetChat(ctx, id)
	if err != nil {
		t.Fatalf("BuscarChat: %v", err)
	}
	if c.AvatarPath != "avatars/1.jpg" {
		t.Errorf("AvatarPath = %q, esperado avatars/1.jpg", c.AvatarPath)
	}

	if err := s.SetAvatar(ctx, id, ""); err != nil {
		t.Fatalf("DefinirAvatar remover: %v", err)
	}
	c, err = s.GetChat(ctx, id)
	if err != nil {
		t.Fatalf("BuscarChat: %v", err)
	}
	if c.AvatarPath != "" {
		t.Errorf("AvatarPath = %q, esperado vazio após remover", c.AvatarPath)
	}
}

func TestSetPhone(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	id := createTestChat(t, s, "Ana", false)

	if err := s.SetPhone(ctx, id, "21 99406-4430"); err != nil {
		t.Fatalf("DefinirTelefone: %v", err)
	}
	c, err := s.GetChat(ctx, id)
	if err != nil {
		t.Fatalf("BuscarChat: %v", err)
	}
	if c.Phone != "21 99406-4430" {
		t.Errorf("Telefone = %q, esperado 21 99406-4430", c.Phone)
	}

	if err := s.SetPhone(ctx, id, ""); err != nil {
		t.Fatalf("DefinirTelefone remover: %v", err)
	}
	c, err = s.GetChat(ctx, id)
	if err != nil {
		t.Fatalf("BuscarChat: %v", err)
	}
	if c.Phone != "" {
		t.Errorf("Telefone = %q, esperado vazio após remover", c.Phone)
	}
}

func TestMessagePagination(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	chatID := createTestChat(t, s, "Paginação", false)

	const total = 55
	for i := 1; i <= total; i++ {
		sentAt := fmt.Sprintf("2026-07-26 %02d:%02d:00", i/60, i%60)
		insertTestMessage(t, s, chatID, sentAt, i, ptr("Vitor"), fmt.Sprintf("mensagem %d", i), "text")
	}

	page1, hasMore, err := s.LastMessagePage(ctx, chatID, 0)
	if err != nil {
		t.Fatalf("UltimaPaginaMensagens: %v", err)
	}
	if len(page1) != 50 {
		t.Fatalf("página 1 tem %d mensagens, esperado 50", len(page1))
	}
	if !hasMore {
		t.Error("esperava temMais=true na primeira página")
	}
	if page1[0].Body != "mensagem 6" {
		t.Errorf("primeira mensagem da última página = %q, esperado 'mensagem 6'", page1[0].Body)
	}
	if page1[len(page1)-1].Body != "mensagem 55" {
		t.Errorf("última mensagem da última página = %q, esperado 'mensagem 55'", page1[len(page1)-1].Body)
	}

	cursorSentAt, cursorSeq, cursorID := page1[0].SentAt, page1[0].Seq, page1[0].ID
	page2, hasMore2, err := s.MessagesBefore(ctx, chatID, cursorSentAt, cursorSeq, cursorID, 0)
	if err != nil {
		t.Fatalf("MensagensAntes: %v", err)
	}
	if len(page2) != 5 {
		t.Fatalf("página 2 tem %d mensagens, esperado 5", len(page2))
	}
	if hasMore2 {
		t.Error("não esperava mais mensagens além da página 2")
	}
	if page2[0].Body != "mensagem 1" || page2[len(page2)-1].Body != "mensagem 5" {
		t.Errorf("página 2 fora de ordem: primeira=%q última=%q", page2[0].Body, page2[len(page2)-1].Body)
	}

	for _, c := range []struct {
		pedido, want int
	}{{10, 10}, {0, 50}, {-3, 50}, {10000, total}} {
		page, hasMore, err := s.LastMessagePage(ctx, chatID, c.pedido)
		if err != nil {
			t.Fatalf("UltimaPaginaMensagens(%d): %v", c.pedido, err)
		}
		if len(page) != c.want {
			t.Errorf("limite %d devolveu %d mensagens, esperado %d", c.pedido, len(page), c.want)
		}
		if c.pedido == 10000 && hasMore {
			t.Error("limite acima do total não deveria sinalizar mais mensagens")
		}
	}
}

func insertTestAttachment(t *testing.T, s *Store, chatID int64, filename, kind, sha string, tamanho int64) int64 {
	t.Helper()
	res, err := s.DB().Exec(`INSERT INTO attachments (chat_id, filename, sha256, media_kind, size_bytes, stored_path)
		VALUES (?, ?, ?, ?, ?, ?)`, chatID, filename, sha, kind, tamanho, "x/"+sha)
	if err != nil {
		t.Fatalf("inserir anexo: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func TestListAttachments_filtersByKind(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	chatID := createTestChat(t, s, "Ana", false)
	ana := "Ana"

	kinds := []struct {
		kind string
		sha  string
	}{{"image", "a1"}, {"video", "a2"}, {"sticker", "a3"}, {"voice", "a4"}, {"document", "a5"}}
	for i, k := range kinds {
		attachmentID := insertTestAttachment(t, s, chatID, k.kind+".bin", k.kind, k.sha, int64(1024*(i+1)))
		sentAt := fmt.Sprintf("2026-07-2%d 10:00:00", i)
		insertTestMessage(t, s, chatID, sentAt, i, &ana, "", "media")
		if _, err := s.DB().Exec(`UPDATE messages SET attachment_id = ? WHERE chat_id = ? AND seq = ?`,
			attachmentID, chatID, i); err != nil {
			t.Fatalf("ligar anexo à mensagem: %v", err)
		}
	}

	cases := map[string][]string{
		"fotos":      {"image"},
		"videos":     {"video", "gif"},
		"documentos": {"document", "contact"},
	}
	want := map[string]int{"fotos": 1, "videos": 1, "documentos": 1}
	for aba, kinds := range cases {
		attachments, err := s.ListAttachments(ctx, chatID, kinds, "", 0, 0)
		if err != nil {
			t.Fatalf("ListarAnexos(%s): %v", aba, err)
		}
		if len(attachments) != want[aba] {
			t.Errorf("aba %s: %d anexos, esperado %d", aba, len(attachments), want[aba])
		}
		for _, a := range attachments {
			if a.SizeBytes == 0 || a.SentAt == "" {
				t.Errorf("aba %s: anexo %q sem size_bytes/sent_at (cards da galeria dependem deles)", aba, a.Filename)
			}
		}
	}

	if attachments, err := s.ListAttachments(ctx, chatID, nil, "", 0, 0); err != nil || len(attachments) != 0 {
		t.Errorf("ListarAnexos com tipos vazio = %d anexos, err=%v; esperado 0 e nil", len(attachments), err)
	}
}

// A sticker sent many times is one attachment (dedupe by sha256), and the gallery must
// not repeat it once per message.
func TestListAttachments_doesNotRepeatResentAttachment(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	chatID := createTestChat(t, s, "Ana", false)
	ana := "Ana"

	attachmentID := insertTestAttachment(t, s, chatID, "sticker.webp", "sticker", "s1", 2048)
	for i, sentAt := range []string{"2026-07-20 10:00:00", "2026-07-21 11:00:00", "2026-07-22 12:00:00"} {
		insertTestMessage(t, s, chatID, sentAt, i, &ana, "", "media")
		if _, err := s.DB().Exec(`UPDATE messages SET attachment_id = ? WHERE chat_id = ? AND seq = ?`,
			attachmentID, chatID, i); err != nil {
			t.Fatalf("ligar anexo à mensagem: %v", err)
		}
	}

	attachments, err := s.ListAttachments(ctx, chatID, []string{"sticker"}, "", 0, 0)
	if err != nil {
		t.Fatalf("ListAttachments: %v", err)
	}
	if len(attachments) != 1 {
		t.Fatalf("figurinha reenviada 3x apareceu %d vezes na galeria, esperado 1", len(attachments))
	}
	if attachments[0].SentAt != "2026-07-22 12:00:00" {
		t.Errorf("sent_at = %q, esperado o envio mais recente (2026-07-22 12:00:00)", attachments[0].SentAt)
	}
}

func TestListMessagesWithLink(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	chatID := createTestChat(t, s, "Ana", false)
	ana := "Ana"

	insertTestMessage(t, s, chatID, "2026-07-20 10:00:00", 1, &ana, "olha isso https://exemplo.com/a", "text")
	insertTestMessage(t, s, chatID, "2026-07-21 10:00:00", 2, &ana, "sem link nenhum aqui", "text")
	insertTestMessage(t, s, chatID, "2026-07-22 10:00:00", 3, &ana, "dois: http://a.com e https://b.com", "text")

	msgs, err := s.ListMessagesWithLink(ctx, chatID)
	if err != nil {
		t.Fatalf("ListarMensagensComLink: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("%d mensagens com link, esperado 2", len(msgs))
	}
	if msgs[0].SentAt != "2026-07-22 10:00:00" {
		t.Errorf("primeira mensagem = %s, esperado a mais recente (2026-07-22)", msgs[0].SentAt)
	}
}

func TestArchiveChat(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	ana := createTestChat(t, s, "Ana", false)
	bia := createTestChat(t, s, "Bia", false)
	sender := "Ana"
	insertTestMessage(t, s, ana, "2026-07-26 10:00:00", 1, &sender, "oi", "text")
	insertTestMessage(t, s, bia, "2026-07-26 11:00:00", 1, &sender, "oi", "text")

	if err := s.SetArchived(ctx, ana, true); err != nil {
		t.Fatalf("DefinirArquivado: %v", err)
	}

	normais, err := s.ListChats(ctx, ChatFilter{})
	if err != nil {
		t.Fatalf("ListarChats: %v", err)
	}
	if len(normais) != 1 || normais[0].ID != bia {
		t.Errorf("lista normal = %d chats, esperado só Bia", len(normais))
	}

	archived, err := s.ListChats(ctx, ChatFilter{Archived: true})
	if err != nil {
		t.Fatalf("ListarChats arquivadas: %v", err)
	}
	if len(archived) != 1 || archived[0].ID != ana || !archived[0].Archived {
		t.Errorf("lista arquivada = %+v, esperado só Ana com Arquivado=true", archived)
	}

	if found, _ := s.ListChats(ctx, ChatFilter{Search: "Ana"}); len(found) != 0 {
		t.Errorf("busca na lista principal trouxe %d arquivadas, esperado 0", len(found))
	}

	if n, err := s.CountArchived(ctx); err != nil || n != 1 {
		t.Errorf("ContarArquivadas = %d (err=%v), esperado 1", n, err)
	}

	if err := s.SetArchived(ctx, ana, false); err != nil {
		t.Fatalf("desarquivar: %v", err)
	}
	if n, _ := s.CountArchived(ctx); n != 0 {
		t.Errorf("ContarArquivadas após desarquivar = %d, esperado 0", n)
	}
}

func TestDeleteChat(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	ana := createTestChat(t, s, "Ana", false)
	bia := createTestChat(t, s, "Bia", false)
	sender := "Ana"
	insertTestMessage(t, s, ana, "2026-07-26 10:00:00", 1, &sender, "oi", "text")
	insertTestMessage(t, s, bia, "2026-07-26 11:00:00", 1, &sender, "oi", "text")
	insertTestAttachment(t, s, ana, "foto.jpg", "image", "aaa", 100)
	insertTestAttachment(t, s, ana, "video.mp4", "video", "bbb", 200)
	insertTestAttachment(t, s, bia, "outra.jpg", "image", "ccc", 300)

	if _, err := s.DB().Exec(`INSERT INTO imports (path, sha256, chat_id, status, started_at)
		VALUES ('Ana.zip', 'hash-ana', ?, 'done', '2026-07-26 10:00:00')`, ana); err != nil {
		t.Fatalf("inserir import: %v", err)
	}
	if _, err := s.DB().Exec(`INSERT INTO seen_files (path, size_bytes, mtime, state, last_seen)
		VALUES ('Ana.zip', 10, '2026-07-26 10:00:00', 'done', '2026-07-26 10:00:00')`); err != nil {
		t.Fatalf("inserir seen_file: %v", err)
	}

	mediaFiles, inbox, err := s.DeleteChat(ctx, ana)
	if err != nil {
		t.Fatalf("ApagarChat: %v", err)
	}
	if len(mediaFiles) != 2 {
		t.Errorf("devolveu %d caminhos de mídia, esperado 2 (o handler precisa deles pra apagar do disco)", len(mediaFiles))
	}
	if len(inbox) != 1 || inbox[0] != "Ana.zip" {
		t.Errorf("caminhos na inbox = %v, esperado [Ana.zip]", inbox)
	}

	if _, err := s.GetChat(ctx, ana); err == nil {
		t.Error("chat apagado ainda existe")
	}

	var n int
	s.DB().QueryRow(`SELECT COUNT(*) FROM messages WHERE chat_id = ?`, ana).Scan(&n)
	if n != 0 {
		t.Errorf("%d mensagens sobraram do chat apagado", n)
	}
	s.DB().QueryRow(`SELECT COUNT(*) FROM attachments WHERE chat_id = ?`, ana).Scan(&n)
	if n != 0 {
		t.Errorf("%d anexos sobraram do chat apagado", n)
	}
	s.DB().QueryRow(`SELECT COUNT(*) FROM attachments WHERE chat_id = ?`, bia).Scan(&n)
	if n != 1 {
		t.Errorf("o outro chat perdeu anexos: %d, esperado 1", n)
	}

	s.DB().QueryRow(`SELECT COUNT(*) FROM imports WHERE path = 'Ana.zip'`).Scan(&n)
	if n != 0 {
		t.Errorf("%d linhas em imports sobraram, esperado 0", n)
	}

	var estado string
	if err := s.DB().QueryRow(`SELECT state FROM seen_files WHERE path = 'Ana.zip'`).Scan(&estado); err != nil {
		t.Fatalf("seen_files sumiu (a conversa voltaria na próxima varredura): %v", err)
	}
	if estado != StateIgnored {
		t.Errorf("seen_files.state = %q, esperado %q", estado, StateIgnored)
	}
}

func TestChatListFilters(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	sender := "Ana"

	direto := createTestChat(t, s, "Ana", false)
	group := createTestChat(t, s, "Time", true)
	outro := createTestChat(t, s, "Bruno", false)
	for i, id := range []int64{direto, group, outro} {
		insertTestMessage(t, s, id, fmt.Sprintf("2026-07-2%d 10:00:00", i), 1, &sender, "oi", "text")
	}

	if err := s.SetFavorite(ctx, direto, true); err != nil {
		t.Fatalf("DefinirFavorito: %v", err)
	}

	favoritas, err := s.ListChats(ctx, ChatFilter{Favorites: true})
	if err != nil || len(favoritas) != 1 || favoritas[0].ID != direto {
		t.Errorf("filtro Favoritas = %+v (err=%v), esperado só Ana", favoritas, err)
	}
	groups, err := s.ListChats(ctx, ChatFilter{Groups: true})
	if err != nil || len(groups) != 1 || groups[0].ID != group {
		t.Errorf("filtro Grupos = %+v (err=%v), esperado só Time", groups, err)
	}

	if n, _ := s.CountByFilter(ctx, ChatFilter{Favorites: true}); n != 1 {
		t.Errorf("ContarPorFiltro(Favoritas) = %d, esperado 1", n)
	}
	if n, _ := s.CountByFilter(ctx, ChatFilter{Groups: true}); n != 1 {
		t.Errorf("ContarPorFiltro(Grupos) = %d, esperado 1", n)
	}

	todos, _ := s.ListChats(ctx, ChatFilter{})
	if todos[0].ID == direto {
		t.Fatal("cenário inválido: Ana já estaria no topo sem fixar")
	}
	if err := s.SetPinned(ctx, direto, true); err != nil {
		t.Fatalf("DefinirFixado: %v", err)
	}
	todos, _ = s.ListChats(ctx, ChatFilter{})
	if todos[0].ID != direto || !todos[0].Pinned {
		t.Errorf("fixada deveria vir primeiro, veio %+v", todos[0])
	}
}

func TestListas(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	sender := "Ana"
	ana := createTestChat(t, s, "Ana", false)
	bruno := createTestChat(t, s, "Bruno", false)
	insertTestMessage(t, s, ana, "2026-07-26 10:00:00", 1, &sender, "oi", "text")
	insertTestMessage(t, s, bruno, "2026-07-26 11:00:00", 1, &sender, "oi", "text")

	trabalho, err := s.CreateList(ctx, "Trabalho")
	if err != nil {
		t.Fatalf("CriarLista: %v", err)
	}
	if _, err := s.CreateList(ctx, "Trabalho"); err == nil {
		t.Error("nome duplicado deveria violar o UNIQUE")
	}

	if err := s.SetChatInList(ctx, ana, trabalho, true); err != nil {
		t.Fatalf("DefinirChatNaLista: %v", err)
	}
	naLista, err := s.ListChats(ctx, ChatFilter{ListID: trabalho})
	if err != nil || len(naLista) != 1 || naLista[0].ID != ana {
		t.Errorf("filtro por lista = %+v (err=%v), esperado só Ana", naLista, err)
	}

	lists, _ := s.ListLists(ctx)
	if len(lists) != 1 || lists[0].Total != 1 {
		t.Errorf("ListarListas = %+v, esperado Trabalho com total 1", lists)
	}

	assoc, err := s.ListAssociations(ctx)
	if err != nil || !assoc[ana][trabalho] || assoc[bruno][trabalho] {
		t.Errorf("AssociacoesDeListas = %+v (err=%v)", assoc, err)
	}

	if err := s.SetChatInList(ctx, ana, trabalho, false); err != nil {
		t.Fatalf("remover da lista: %v", err)
	}
	if naLista, _ := s.ListChats(ctx, ChatFilter{ListID: trabalho}); len(naLista) != 0 {
		t.Errorf("lista deveria estar vazia, veio %+v", naLista)
	}

	if err := s.DeleteList(ctx, trabalho); err != nil {
		t.Fatalf("ApagarLista: %v", err)
	}
	if todos, _ := s.ListChats(ctx, ChatFilter{}); len(todos) != 2 {
		t.Errorf("apagar a lista mexeu nas conversas: %d, esperado 2", len(todos))
	}
}

// The merge picker must never offer the conversation itself.
func TestSearchMergeCandidates(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	atual := createTestChat(t, s, "Ana Souza", false)
	candidata := createTestChat(t, s, "Ana Souza 2025", false)
	group := createTestChat(t, s, "Ana no grupo", true)
	archived := createTestChat(t, s, "Ana arquivada", false)
	if err := s.SetArchived(ctx, archived, true); err != nil {
		t.Fatalf("arquivar: %v", err)
	}

	candidatas, err := s.SearchMergeCandidates(ctx, atual, false, "")
	if err != nil {
		t.Fatalf("SearchMergeCandidates: %v", err)
	}
	var ids []int64
	for _, c := range candidatas {
		ids = append(ids, c.ID)
	}
	achouCandidata, achouGrupo, foundArchived, achouAtual := false, false, false, false
	for _, id := range ids {
		switch id {
		case candidata:
			achouCandidata = true
		case group:
			achouGrupo = true
		case archived:
			foundArchived = true
		case atual:
			achouAtual = true
		}
	}
	if !achouCandidata {
		t.Error("candidata individual não apareceu na busca")
	}
	if achouGrupo {
		t.Error("grupo apareceu como candidato de uma conversa individual")
	}
	if foundArchived {
		t.Error("conversa arquivada apareceu como candidata")
	}
	if achouAtual {
		t.Error("a própria conversa apareceu como candidata a si mesma")
	}

	filtradas, err := s.SearchMergeCandidates(ctx, atual, false, "2025")
	if err != nil || len(filtradas) != 1 || filtradas[0].ID != candidata {
		t.Errorf("busca por nome = %+v (err=%v), esperado só a candidata", filtradas, err)
	}
}

// Without ESCAPE, typing "_" matches any character and "%" matches everything, so the
// search silently stops filtering.
func TestEscapeLike(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	createTestChat(t, s, "Ana Souza", false)
	createTestChat(t, s, "Grupo 100%", false)
	createTestChat(t, s, "ana_souza", false)

	cases := []struct {
		search string
		want   []string
	}{
		{"100%", []string{"Grupo 100%"}},
		{"_", []string{"ana_souza"}},  // literal, not "any character"
		{"%", []string{"Grupo 100%"}}, // literal, not "everything"
		{"ana", []string{"Ana Souza", "ana_souza"}},
	}
	for _, c := range cases {
		chats, err := s.ListChats(ctx, ChatFilter{Search: c.search})
		if err != nil {
			t.Fatalf("ListChats(%q): %v", c.search, err)
		}
		names := make([]string, 0, len(chats))
		for _, chat := range chats {
			names = append(names, chat.Name)
		}
		slices.Sort(names)
		want := slices.Clone(c.want)
		slices.Sort(want)
		if !slices.Equal(names, want) {
			t.Errorf("busca %q devolveu %v, esperado %v", c.search, names, want)
		}
	}
}

// (sent_at, seq) is not unique across imports, so a two-part cursor skips the tied row
// that falls on a page boundary.
func TestMessagesBefore_breaksTiesByID(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	ana := "Ana"
	chatID := createTestChat(t, s, "Empate", false)

	for i := 1; i <= 4; i++ {
		if _, err := s.DB().Exec(
			`INSERT INTO messages (chat_id, sent_at, seq, sender, body, kind, hash) VALUES (?,?,?,?,?,?,?)`,
			chatID, "2026-07-20 10:00:00", 1, ana, fmt.Sprintf("empatada %d", i), "text",
			fmt.Sprintf("hash-%d", i)); err != nil {
			t.Fatal(err)
		}
	}

	vistas := map[string]bool{}
	page, hasMore, err := s.LastMessagePage(ctx, chatID, 2)
	if err != nil {
		t.Fatal(err)
	}
	for {
		for _, m := range page {
			if vistas[m.Body] {
				t.Fatalf("mensagem %q apareceu duas vezes na rolagem", m.Body)
			}
			vistas[m.Body] = true
		}
		if !hasMore {
			break
		}
		first := page[0]
		page, hasMore, err = s.MessagesBefore(ctx, chatID, first.SentAt, first.Seq, first.ID, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
	}

	if len(vistas) != 4 {
		t.Errorf("a rolagem alcançou %d das 4 mensagens: %v", len(vistas), vistas)
	}
}

// The gallery pages through attachments; a cursor that isn't strictly "before" the last
// item of a page would repeat or skip it at the page boundary.
func TestListAttachments_paginates(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	chatID := createTestChat(t, s, "Ana", false)
	ana := "Ana"

	for i := 0; i < 5; i++ {
		id := insertTestAttachment(t, s, chatID, fmt.Sprintf("img%d.jpg", i), "image", fmt.Sprintf("h%d", i), 100)
		insertTestMessage(t, s, chatID, fmt.Sprintf("2026-07-2%d 10:00:00", i), i, &ana, "", "media")
		if _, err := s.DB().Exec(`UPDATE messages SET attachment_id = ? WHERE chat_id = ? AND seq = ?`, id, chatID, i); err != nil {
			t.Fatalf("ligar anexo: %v", err)
		}
	}

	first, err := s.ListAttachments(ctx, chatID, []string{"image"}, "", 0, 2)
	if err != nil || len(first) != 2 {
		t.Fatalf("primeira página = %d anexos, err=%v; esperado 2", len(first), err)
	}
	last := first[len(first)-1]
	second, err := s.ListAttachments(ctx, chatID, []string{"image"}, last.SentAt, last.ID, 2)
	if err != nil || len(second) != 2 {
		t.Fatalf("segunda página = %d anexos, err=%v; esperado 2", len(second), err)
	}
	for _, a := range second {
		for _, b := range first {
			if a.ID == b.ID {
				t.Errorf("anexo %d repetido entre as páginas", a.ID)
			}
		}
	}
	if second[0].SentAt >= last.SentAt {
		t.Errorf("segunda página começou em %q, que não é anterior a %q", second[0].SentAt, last.SentAt)
	}
}
