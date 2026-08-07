package web

import (
	"fmt"
	"testing"
	"time"

	"github.com/wallacepnts/vaultzap/internal/render"
	"github.com/wallacepnts/vaultzap/internal/store"
)

func TestBuildMessageViews(t *testing.T) {
	vitor := "Vitor"
	ana := "Ana"
	agora := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)

	messages := []store.Message{
		{ID: 1, SentAt: "2026-07-26 14:32:00", Sender: &vitor, Body: "oi", Kind: "text"},
		{ID: 2, SentAt: "2026-07-26 14:33:00", Sender: &vitor, Body: "tudo bem?", Kind: "text"},
		{ID: 3, SentAt: "2026-07-26 14:34:00", Sender: &ana, Body: "tudo!", Kind: "text"},
		{ID: 4, SentAt: "2026-07-27 09:00:00", Sender: &ana, Body: "bom dia", Kind: "text"},
	}

	vistas := buildMessageViews(messages, conversationContext{Owner: "Vitor", IsGroup: false, ChatID: 1}, agora, 0)

	if len(vistas) != 4 {
		t.Fatalf("esperava 4 mensagens, veio %d", len(vistas))
	}

	for i, v := range vistas {
		if v.ShowSenderName {
			t.Errorf("mensagem %d: MostrarNomeRemetente deveria ser false numa conversa 1:1 (ehGrupo=false)", i)
		}
	}

	if vistas[0].DateDivider != "ONTEM" {
		t.Errorf("divisor da primeira mensagem = %q, esperado ONTEM", vistas[0].DateDivider)
	}
	if !vistas[0].FirstInBlock {
		t.Error("primeira mensagem do arquivo deveria iniciar um bloco")
	}
	if !vistas[0].Mine {
		t.Error("mensagem de Vitor deveria ser MinhaMensagem (dono=Vitor)")
	}

	if vistas[1].FirstInBlock {
		t.Error("segunda mensagem (mesmo remetente, mesmo dia) não deveria iniciar bloco novo")
	}
	if vistas[1].DateDivider != "" {
		t.Errorf("segunda mensagem não deveria ter divisor, veio %q", vistas[1].DateDivider)
	}

	if !vistas[2].FirstInBlock {
		t.Error("terceira mensagem (remetente diferente) deveria iniciar bloco novo")
	}
	if vistas[2].Mine {
		t.Error("mensagem de Ana não deveria ser MinhaMensagem")
	}

	if vistas[3].DateDivider != "HOJE" {
		t.Errorf("divisor da quarta mensagem = %q, esperado HOJE", vistas[3].DateDivider)
	}
	if !vistas[3].FirstInBlock {
		t.Error("mensagem após divisor de data deveria iniciar bloco novo mesmo com o mesmo remetente")
	}
	if vistas[3].ShortTime != "09:00" {
		t.Errorf("HoraCurta = %q, esperado 09:00", vistas[3].ShortTime)
	}
}

func TestBuildMessageViews_senderNameOnlyInGroup(t *testing.T) {
	vitor, ana := "Vitor", "Ana"
	agora := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	messages := []store.Message{
		{ID: 1, SentAt: "2026-07-26 14:32:00", Sender: &vitor, Body: "oi", Kind: "text"},
		{ID: 2, SentAt: "2026-07-26 14:33:00", Sender: &ana, Body: "e aí", Kind: "text"},
	}

	semGrupo := buildMessageViews(messages, conversationContext{Owner: "Vitor", IsGroup: false, ChatID: 1}, agora, 0)
	if semGrupo[1].ShowSenderName {
		t.Error("conversa 1:1 (ehGrupo=false) não deveria mostrar nome do remetente")
	}

	comGrupo := buildMessageViews(messages, conversationContext{Owner: "Vitor", IsGroup: true, ChatID: 1}, agora, 0)
	if !comGrupo[1].ShowSenderName {
		t.Error("mensagem de Ana em grupo, primeira do bloco, deveria mostrar nome do remetente")
	}
	if comGrupo[0].ShowSenderName {
		t.Error("mensagem própria (MinhaMensagem) não deveria mostrar nome do remetente mesmo em grupo")
	}
}

func TestDateLabel_weekdayWithinAWeek(t *testing.T) {
	vitor := "Vitor"
	agora := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC) // Monday

	messages := []store.Message{
		{ID: 1, SentAt: "2026-07-24 10:00:00", Sender: &vitor, Body: "oi", Kind: "text"}, // Friday, 3 days ago
	}

	vistas := buildMessageViews(messages, conversationContext{Owner: "Vitor", IsGroup: false, ChatID: 1}, agora, 0)
	if vistas[0].DateDivider != "sexta-feira" {
		t.Errorf("DivisorData = %q, esperado sexta-feira", vistas[0].DateDivider)
	}
}

func TestDateLabel_beyondAWeekUsesFullDate(t *testing.T) {
	vitor := "Vitor"
	agora := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)

	messages := []store.Message{
		{ID: 1, SentAt: "2026-07-10 10:00:00", Sender: &vitor, Body: "oi", Kind: "text"},
	}

	vistas := buildMessageViews(messages, conversationContext{Owner: "Vitor", IsGroup: false, ChatID: 1}, agora, 0)
	if vistas[0].DateDivider != "10 de julho de 2026" {
		t.Errorf("DivisorData = %q, esperado 10 de julho de 2026", vistas[0].DateDivider)
	}
}

func TestDateLabel_localeEN(t *testing.T) {
	vitor := "Vitor"
	agora := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC) // Monday

	messages := []store.Message{
		{ID: 1, SentAt: "2026-07-27 14:32:00", Sender: &vitor, Body: "hi", Kind: "text"},  // today
		{ID: 2, SentAt: "2026-07-26 09:00:00", Sender: &vitor, Body: "hey", Kind: "text"}, // yesterday
		{ID: 3, SentAt: "2026-07-24 10:00:00", Sender: &vitor, Body: "yo", Kind: "text"},  // Friday, 3 days ago
		{ID: 4, SentAt: "2026-07-10 10:00:00", Sender: &vitor, Body: "old", Kind: "text"}, // full date
	}

	vistas := buildMessageViews(messages, conversationContext{Owner: "Vitor", ChatID: 1, Locale: render.LocaleEN}, agora, 0)

	if vistas[0].DateDivider != "TODAY" || vistas[0].ShortTime != "2:32 PM" {
		t.Errorf("hoje = %q / %q, esperado TODAY / 2:32 PM", vistas[0].DateDivider, vistas[0].ShortTime)
	}
	if vistas[1].DateDivider != "YESTERDAY" {
		t.Errorf("ontem = %q, esperado YESTERDAY", vistas[1].DateDivider)
	}
	if vistas[2].DateDivider != "Friday" {
		t.Errorf("dia da semana = %q, esperado Friday", vistas[2].DateDivider)
	}
	if vistas[3].DateDivider != "July 10, 2026" {
		t.Errorf("data completa = %q, esperado July 10, 2026", vistas[3].DateDivider)
	}
}

func TestParticipantNickname(t *testing.T) {
	numero := "+55 21 99581-0912"
	eu := "Wallace"
	agora := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	messages := []store.Message{
		{ID: 1, SentAt: "2026-07-26 14:32:00", Sender: &numero, Body: "oi", Kind: "text"},
		{ID: 2, SentAt: "2026-07-26 14:33:00", Sender: &eu, Body: "oi", Kind: "text"},
	}

	ctx := conversationContext{
		Owner:     "Wallace",
		IsGroup:   true,
		ChatID:    1,
		Nicknames: map[string]string{numero: "Raphael"},
	}
	vistas := buildMessageViews(messages, ctx, agora, 0)

	if vistas[0].SenderName != "Raphael" {
		t.Errorf("SenderNome = %q, esperado Raphael (apelido do participante)", vistas[0].SenderName)
	}
	if vistas[0].Sender == nil || *vistas[0].Sender != numero {
		t.Error("o Sender original do export não pode ser alterado — é a chave do hash de dedupe")
	}
	if vistas[0].SenderClass != avatarClass("Raphael") {
		t.Error("a cor deve seguir o nome exibido, senão fica a cor de um nome que não aparece mais")
	}

	semApelido := buildMessageViews(messages, conversationContext{Owner: "Wallace", IsGroup: true, ChatID: 1}, agora, 0)
	if semApelido[0].SenderName != numero {
		t.Errorf("SenderNome sem apelido = %q, esperado o original %q", semApelido[0].SenderName, numero)
	}
}

// Consecutive stickers from one sender form a single strip; a message in between, a change
// of sender or a new day starts another one.
func TestMarkStickerRuns(t *testing.T) {
	sticker := func(v *MessageView) { v.AttachmentMediaKind = "sticker" }
	views := make([]MessageView, 6)
	sticker(&views[0])
	sticker(&views[1])
	sticker(&views[2])
	views[2].FirstInBlock = true // outro remetente
	sticker(&views[3])
	// views[4] is text, views[5] is a sticker again.
	sticker(&views[5])

	markStickerRuns(views)

	inicios, fins := []int{}, []int{}
	for i, v := range views {
		if v.StickerRunStart {
			inicios = append(inicios, i)
		}
		if v.StickerRunEnd {
			fins = append(fins, i)
		}
	}
	if fmt.Sprint(inicios) != "[0 2 5]" {
		t.Errorf("inícios de faixa = %v, esperado [0 2 5]", inicios)
	}
	if fmt.Sprint(fins) != "[1 3 5]" {
		t.Errorf("fins de faixa = %v, esperado [1 3 5]", fins)
	}
}

// The profile name replaces the export's sender for whoever is the owner, but a nickname
// set for that same sender still wins — it is the more specific choice.
func TestDisplayName_ownerUsesProfileName(t *testing.T) {
	ctx := conversationContext{Owner: "Wallace Pontes", OwnerName: "Wallace P."}
	if got := ctx.displayName("Wallace Pontes"); got != "Wallace P." {
		t.Errorf("dono = %q, esperado o nome do perfil", got)
	}
	if got := ctx.displayName("Ana"); got != "Ana" {
		t.Errorf("outro remetente = %q, não deveria mudar", got)
	}

	ctx.Nicknames = map[string]string{"Wallace Pontes": "Eu mesmo"}
	if got := ctx.displayName("Wallace Pontes"); got != "Eu mesmo" {
		t.Errorf("apelido explícito = %q, deveria vencer o nome do perfil", got)
	}
}
