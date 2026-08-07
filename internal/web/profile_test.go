package web

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/wallacepnts/vaultzap/internal/config"
	"github.com/wallacepnts/vaultzap/internal/store"
)

func createTestChat(t *testing.T, s *store.Store, name string) int64 {
	t.Helper()
	res, err := s.DB().Exec(
		`INSERT INTO chats (name, is_group, source, created_at) VALUES (?, 0, 'android', '2026-07-26 00:00:00')`, name)
	if err != nil {
		t.Fatalf("create test chat: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func createTestGroup(t *testing.T, s *store.Store, name string) int64 {
	t.Helper()
	res, err := s.DB().Exec(
		`INSERT INTO chats (name, is_group, source, created_at) VALUES (?, 1, 'ios', '2026-07-26 00:00:00')`, name)
	if err != nil {
		t.Fatalf("create test group: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func insertTestMessage(t *testing.T, s *store.Store, chatID int64, seq int, sender, body string) {
	t.Helper()
	_, err := s.DB().Exec(
		`INSERT INTO messages (chat_id, sent_at, seq, sender, body, kind, hash) VALUES (?, ?, ?, ?, ?, 'text', ?)`,
		chatID, "2026-07-26 10:00:00", seq, sender, body, fmt.Sprintf("%d-%d", chatID, seq))
	if err != nil {
		t.Fatalf("insert test message: %v", err)
	}
}

func TestRenameChat_handler(t *testing.T) {
	h := openTestHandler(t, config.Config{})
	id := createTestChat(t, h.store, "Ana")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/chats/%d/renomear", id), strings.NewReader("name=Ana+Souza"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, corpo: %s", rec.Code, rec.Body.String())
	}

	chat, err := h.store.GetChat(context.Background(), id)
	if err != nil {
		t.Fatalf("GetChat: %v", err)
	}
	if chat.Name != "Ana Souza" {
		t.Errorf("nome = %q, esperado Ana Souza", chat.Name)
	}
}

func TestRenameChat_handler_collisionReturns409(t *testing.T) {
	h := openTestHandler(t, config.Config{})
	createTestChat(t, h.store, "Bruno")
	id := createTestChat(t, h.store, "Ana")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/chats/%d/renomear", id), strings.NewReader("name=Bruno"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, esperado 409", rec.Code)
	}
}

func TestSetAvatar_handler_rejectsInvalidFormat(t *testing.T) {
	h := openTestHandler(t, config.Config{MediaDir: t.TempDir()})
	id := createTestChat(t, h.store, "Ana")

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("photo", "avatar.txt")
	part.Write([]byte("não é uma imagem"))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/chats/%d/avatar", id), &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, esperado 415, corpo: %s", rec.Code, rec.Body.String())
	}
}

func TestSetAvatar_handler_pngAcceptedAndServed(t *testing.T) {
	h := openTestHandler(t, config.Config{MediaDir: t.TempDir()})
	id := createTestChat(t, h.store, "Ana")

	pngSignature := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("photo", "avatar.png")
	part.Write(pngSignature)
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/chats/%d/avatar", id), &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, corpo: %s", rec.Code, rec.Body.String())
	}

	chat, err := h.store.GetChat(context.Background(), id)
	if err != nil {
		t.Fatalf("GetChat: %v", err)
	}
	if chat.AvatarPath == "" {
		t.Fatal("esperava AvatarPath preenchido após upload")
	}

	reqGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/chats/%d/avatar", id), nil)
	recGet := httptest.NewRecorder()
	h.Routes().ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("GET avatar status = %d", recGet.Code)
	}

	reqDel := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/chats/%d/avatar", id), nil)
	recDel := httptest.NewRecorder()
	h.Routes().ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusOK {
		t.Fatalf("DELETE avatar status = %d, corpo: %s", recDel.Code, recDel.Body.String())
	}
	chat, err = h.store.GetChat(context.Background(), id)
	if err != nil {
		t.Fatalf("GetChat: %v", err)
	}
	if chat.AvatarPath != "" {
		t.Errorf("AvatarPath = %q, esperado vazio após remover", chat.AvatarPath)
	}
}

func TestOwnerCandidates_groupDoesNotListGroupName(t *testing.T) {
	h := openTestHandler(t, config.Config{})
	ctx := context.Background()

	groupID := createTestGroup(t, h.store, "Reunião da treta")
	insertTestMessage(t, h.store, groupID, 1, "Ana", "oi")
	insertTestMessage(t, h.store, groupID, 2, "Bruno", "e aí")
	insertTestMessage(t, h.store, groupID, 3, "Reunião da treta", "Você saiu")

	group, err := h.store.GetChat(ctx, groupID)
	if err != nil {
		t.Fatalf("GetChat: %v", err)
	}
	candidates, err := h.ownerCandidates(ctx, group)
	if err != nil {
		t.Fatalf("ownerCandidates: %v", err)
	}
	for _, c := range candidates {
		if c == "Reunião da treta" {
			t.Errorf("o nome do grupo apareceu como candidato a dono: %v", candidates)
		}
	}
	if len(candidates) != 2 {
		t.Errorf("candidatos = %v, esperado só Ana e Bruno", candidates)
	}

	directID := createTestChat(t, h.store, "Ana")
	insertTestMessage(t, h.store, directID, 1, "Ana", "oi")
	insertTestMessage(t, h.store, directID, 2, "Wallace", "oi")

	direct, err := h.store.GetChat(ctx, directID)
	if err != nil {
		t.Fatalf("GetChat: %v", err)
	}
	candidates, err = h.ownerCandidates(ctx, direct)
	if err != nil {
		t.Fatalf("ownerCandidates: %v", err)
	}
	if len(candidates) != 2 {
		t.Errorf("candidatos em 1:1 = %v, esperado Ana e Wallace (o nome do chat não pode ser filtrado)", candidates)
	}
}

func TestRename_wholeNameInOneField(t *testing.T) {
	h := openTestHandler(t, config.Config{})
	id := createTestGroup(t, h.store, "Reunião da treta")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/chats/%d/renomear", id),
		strings.NewReader("name=Reuni%C3%A3o+da+treta+2026"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, corpo: %s", rec.Code, rec.Body.String())
	}
	if chat := mustGetChat(t, h, id); chat.Name != "Reunião da treta 2026" {
		t.Errorf("nome = %q, esperado o nome inteiro", chat.Name)
	}
}

func TestRenameChat_missingPhoneFieldKeepsPhone(t *testing.T) {
	h := openTestHandler(t, config.Config{})
	id := createTestChat(t, h.store, "Ana")
	if err := h.store.SetPhone(context.Background(), id, "21 99406-4430"); err != nil {
		t.Fatalf("SetPhone: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/chats/%d/renomear", id),
		strings.NewReader("name=Ana+Souza")) // no phone field in the body
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if chat := mustGetChat(t, h, id); chat.Phone != "21 99406-4430" {
		t.Errorf("telefone = %q, esperado intacto — o campo não foi enviado", chat.Phone)
	}
}

func mustGetChat(t *testing.T, h *Handler, id int64) store.Chat {
	t.Helper()
	chat, err := h.store.GetChat(context.Background(), id)
	if err != nil {
		t.Fatalf("GetChat: %v", err)
	}
	return chat
}

func TestRenameContact_compoundNameIsNotTruncated(t *testing.T) {
	h := openTestHandler(t, config.Config{})
	id := createTestChat(t, h.store, "Ana")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/chats/%d/renomear", id),
		strings.NewReader("name=Ana+Souza+da+Silva&phone=21+99406-4430"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, corpo: %s", rec.Code, rec.Body.String())
	}

	chat := mustGetChat(t, h, id)
	if chat.Name != "Ana Souza da Silva" {
		t.Errorf("nome = %q, esperado Ana Souza da Silva", chat.Name)
	}
	if chat.Phone != "21 99406-4430" {
		t.Errorf("telefone = %q, esperado 21 99406-4430", chat.Phone)
	}
}

func TestFillParticipants(t *testing.T) {
	all := make([]ParticipantView, 0, 100)
	for i := 0; i < 100; i++ {
		n := fmt.Sprintf("+55 11 9000-%04d", i)
		all = append(all, ParticipantView{Original: n, Display: n})
	}

	// Two named contacts and the owner, all at the bottom of the volume order.
	all = append(all, ParticipantView{Original: "z", Display: "Zeca"},
		ParticipantView{Original: "a", Display: "Ana"},
		ParticipantView{Original: "eu", Display: "Wallace", IsOwner: true})

	var data ProfileData
	fillParticipants(&data, all)
	if !data.Participants[0].IsOwner {
		t.Errorf("o dono deveria abrir a lista, veio %q", data.Participants[0].Display)
	}
	if data.Participants[1].Display != "Zeca" || data.Participants[2].Display != "Ana" {
		t.Errorf("os nomeados deveriam vir logo depois, veio %q e %q",
			data.Participants[1].Display, data.Participants[2].Display)
	}
	if data.ParticipantTotal != 103 {
		t.Errorf("total = %d, queria 103", data.ParticipantTotal)
	}
	if len(data.Participants) != maxParticipantsShown {
		t.Errorf("exibidos = %d, queria o teto de %d", len(data.Participants), maxParticipantsShown)
	}
	if data.ParticipantRest != 103-maxParticipantsShown {
		t.Errorf("restantes = %d, queria %d", data.ParticipantRest, 103-maxParticipantsShown)
	}

	fillParticipants(&data, all[:3])
	if data.ParticipantRest != 0 {
		t.Errorf("3 participantes cabem no painel, restantes = %d", data.ParticipantRest)
	}
}

// Letters first, then the "~" of a push name, then numbers — the official app's order.
func TestGroupParticipantsByLetter(t *testing.T) {
	all := []ParticipantView{
		{Original: "+55 21 98245-4494", Display: "+55 21 98245-4494"},
		{Original: "x", Display: "~ Andrew"},
		{Original: "y", Display: "Mathias"},
		{Original: "z", Display: "Gustavo"},
		{Original: "w", Display: "~ Bernardo"},
	}
	groups := groupParticipantsByLetter(all, "")

	var letras []string
	for _, g := range groups {
		letras = append(letras, g.Letter)
	}
	if fmt.Sprint(letras) != "[G M ~ #]" {
		t.Errorf("seções = %v, esperado [G M ~ #]", letras)
	}
	if len(groups[2].Items) != 2 || groups[2].Items[0].Display != "~ Andrew" {
		t.Errorf("seção ~ = %+v, esperada em ordem alfabética", groups[2].Items)
	}

	if g := groupParticipantsByLetter(all, "gust"); len(g) != 1 || len(g[0].Items) != 1 {
		t.Errorf("busca por 'gust' devolveu %+v", g)
	}
}

func TestGalleryTabCounts(t *testing.T) {
	h := openTestHandler(t, config.Config{})
	id := createTestChat(t, h.store, "Ana")

	// video and gif share a tab, audio and voice share another: the count has to add them up.
	for i, kind := range []string{"image", "image", "video", "gif", "voice", "audio", "sticker"} {
		if _, err := h.store.DB().Exec(
			`INSERT INTO attachments (chat_id, filename, sha256, media_kind, stored_path)
			 VALUES (?, ?, ?, ?, '')`, id, fmt.Sprintf("f%d", i), fmt.Sprintf("h%d", i), kind); err != nil {
			t.Fatalf("insert attachment: %v", err)
		}
	}
	insertTestMessage(t, h.store, id, 1, "Ana", "olha https://a.dev e https://b.dev")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/chats/%d/media?aba=fotos", id), nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	for _, want := range []string{"Fotos (2)", "Vídeos (2)", "Áudios (2)", "Figurinhas (1)", "Documentos (0)", "Links (2)"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("faltou %q nas abas", want)
		}
	}
}

// The owner used to be filtered out of the participant list, so searching for your own name
// answered "no results" for someone who is plainly in the group.
func TestGroupParticipants_includesOwnerAsYou(t *testing.T) {
	h := openTestHandler(t, config.Config{})
	id := createTestGroup(t, h.store, "Time")
	if _, err := h.store.DB().Exec(`UPDATE chats SET owner = 'Wallace' WHERE id = ?`, id); err != nil {
		t.Fatalf("definir owner: %v", err)
	}
	insertTestMessage(t, h.store, id, 1, "Wallace", "oi")
	insertTestMessage(t, h.store, id, 2, "Ana", "oi")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/chats/%d/membros?q=wall", id), nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Wallace") {
		t.Fatalf("o dono não apareceu na busca por participante:\n%s", body)
	}
	if !strings.Contains(body, "Você") {
		t.Error("o dono deveria vir marcado como \"Você\"")
	}
	if strings.Contains(body, "button-edit-participant") {
		t.Error("o dono não deveria ter botão de renomear")
	}
}

func TestParticipantPhoto(t *testing.T) {
	h := openTestHandler(t, config.Config{})
	group := createTestGroup(t, h.store, "Time")
	contact := createTestChat(t, h.store, "Ana")
	if _, err := h.store.DB().Exec(`UPDATE chats SET avatar_path = 'avatars/2.png' WHERE id = ?`, contact); err != nil {
		t.Fatalf("dar foto ao contato: %v", err)
	}
	insertTestMessage(t, h.store, group, 1, "+55 11 90000-0000", "oi")
	insertTestMessage(t, h.store, group, 2, "Bruno", "oi")

	form := url.Values{"sender": {"+55 11 90000-0000"}, "chat_id": {fmt.Sprint(contact)}}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/chats/%d/participante/foto", group), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, corpo: %s", rec.Code, rec.Body.String())
	}

	// A linked photo is served by the other chat's own route, never copied.
	want := fmt.Sprintf(`src="/chats/%d/avatar"`, contact)
	if !strings.Contains(rec.Body.String(), want) {
		t.Errorf("o painel não mostrou a foto vinculada (%s)", want)
	}

	// An upload replaces the link, and vice versa: only one of the two is ever set.
	if err := h.store.SetParticipantAvatarPath(context.Background(), group, "+55 11 90000-0000", "avatars/1-7.png"); err != nil {
		t.Fatalf("SetParticipantAvatarPath: %v", err)
	}
	avatars, err := h.store.ParticipantAvatars(context.Background(), group)
	if err != nil {
		t.Fatalf("ParticipantAvatars: %v", err)
	}
	got := avatars["+55 11 90000-0000"]
	if got.Path != "avatars/1-7.png" || got.LinkedChatID != 0 {
		t.Errorf("upload deveria limpar o vínculo, veio %+v", got)
	}
}

// The global gallery is the same handler with no chat filter: chat id 0 all the way down.
func TestAllMediaGallery(t *testing.T) {
	h := openTestHandler(t, config.Config{})
	ana := createTestChat(t, h.store, "Ana")
	bruno := createTestChat(t, h.store, "Bruno")
	for i, chat := range []int64{ana, bruno} {
		res, err := h.store.DB().Exec(
			`INSERT INTO attachments (chat_id, filename, sha256, media_kind, stored_path, size_bytes)
			 VALUES (?, ?, ?, 'image', '', 100)`, chat, fmt.Sprintf("img%d.jpg", i), fmt.Sprintf("h%d", i))
		if err != nil {
			t.Fatalf("inserir anexo: %v", err)
		}
		id, _ := res.LastInsertId()
		insertTestMessage(t, h.store, chat, i, "Ana", "")
		if _, err := h.store.DB().Exec(`UPDATE messages SET attachment_id = ?, kind = 'media' WHERE chat_id = ? AND seq = ?`, id, chat, i); err != nil {
			t.Fatalf("ligar anexo: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/midia", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Fotos (2)") {
		t.Error("a galeria global deveria somar as fotos das duas conversas")
	}
	if strings.Count(body, "gallery-item") != 2 {
		t.Errorf("itens = %d, esperado 2 (um de cada conversa)", strings.Count(body, "gallery-item"))
	}
}

// The name saved in the profile is the UI's VAULTZAP_ME: it decides who counts as the user
// and wins over the env var, so changing it needs no restart.
func TestMyNameOverridesEnv(t *testing.T) {
	h := openTestHandler(t, config.Config{Me: "Do Env"})
	ctx := context.Background()

	if got := h.me(ctx); got != "Do Env" {
		t.Errorf("sem preferência salva, o dono deveria vir do env: %q", got)
	}

	form := strings.NewReader(url.Values{"name": {"Wallace"}}.Encode())
	req := httptest.NewRequest(http.MethodPost, "/eu", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, corpo: %s", rec.Code, rec.Body.String())
	}

	if got := h.me(ctx); got != "Wallace" {
		t.Errorf("a preferência deveria vencer o env, veio %q", got)
	}

	// Clearing goes back to the env var, which is why the row is deleted, not blanked.
	if err := h.store.SetSetting(ctx, store.SettingMe, ""); err != nil {
		t.Fatalf("limpar: %v", err)
	}
	if got := h.me(ctx); got != "Do Env" {
		t.Errorf("limpo, deveria voltar pro env: %q", got)
	}
}

// A group where the owner was never picked still has to recognise you by the profile name,
// or nobody shows as "Você" — and so nobody gets your photo.
func TestGroupParticipants_ownerFromProfileName(t *testing.T) {
	h := openTestHandler(t, config.Config{Me: "Wallace"})
	id := createTestGroup(t, h.store, "Reunião")
	insertTestMessage(t, h.store, id, 1, "Wallace", "oi")
	insertTestMessage(t, h.store, id, 2, "Ana", "oi")

	participants, err := h.groupParticipants(context.Background(), store.Chat{ID: id, IsGroup: true, Name: "Reunião"})
	if err != nil {
		t.Fatalf("groupParticipants: %v", err)
	}
	for _, p := range participants {
		if p.Display == "Wallace" && !p.IsOwner {
			t.Error("sem dono no chat, o nome do perfil deveria marcar você")
		}
		if p.Display == "Ana" && p.IsOwner {
			t.Error("só quem bate com o nome do perfil é o dono")
		}
	}
}

// An attachment's Content-Type comes from the name the export chose, so "x.html" would be
// served as a page on the app's own origin — stored XSS with the whole archive in reach.
func TestInlineMediaType(t *testing.T) {
	casos := []struct {
		mime   string
		want   string
		inline bool
	}{
		{"image/jpeg", "image/jpeg", true},
		{"image/webp", "image/webp", true},
		{"video/mp4", "video/mp4", true},
		{"audio/ogg", "audio/ogg", true},
		{"text/html; charset=utf-8", "application/octet-stream", false},
		{"application/pdf", "application/octet-stream", false},
		{"image/svg+xml", "application/octet-stream", false},
		{"IMAGE/SVG+XML", "application/octet-stream", false},
		{"", "application/octet-stream", false},
	}
	for _, c := range casos {
		got, inline := inlineMediaType(c.mime)
		if got != c.want || inline != c.inline {
			t.Errorf("inlineMediaType(%q) = (%q, %v), esperado (%q, %v)", c.mime, got, inline, c.want, c.inline)
		}
	}
}
