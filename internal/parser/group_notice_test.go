package parser

import (
	"strings"
	"testing"
)

func TestReclassifyGroupNotice(t *testing.T) {
	const group = "Reunião da treta"

	export := strings.Join([]string{
		"[15/05/2018, 10:30:00] " + group + ": Ana criou o grupo \"Reunião da treta\"",
		"[15/05/2018, 10:31:00] " + group + ": Ana adicionou você",
		"[15/05/2018, 10:32:00] Ana: bom dia pessoal",
		"[15/05/2018, 10:33:00] " + group + ": Você saiu",
	}, "\n")

	res, err := Parse(strings.NewReader(export), Options{ChatName: group})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Messages) != 4 {
		t.Fatalf("%d messages, expected 4", len(res.Messages))
	}

	for _, i := range []int{0, 1, 3} {
		m := res.Messages[i]
		if m.Sender != nil {
			t.Errorf("message %d (%q): sender = %q, expected nil", i, m.Body, *m.Sender)
		}
		if m.Kind != "system" {
			t.Errorf("message %d (%q): kind = %q, expected system", i, m.Body, m.Kind)
		}
	}

	if m := res.Messages[2]; m.Sender == nil || *m.Sender != "Ana" || m.Kind != "text" {
		t.Errorf("Ana's real message was altered: %+v", m)
	}
	if !res.IsGroup {
		t.Error("IsGroup = false, expected true (the reclassified notices should feed the inference)")
	}
}

func TestReclassifyGroupNotice_es(t *testing.T) {
	const group = "Amigos del trabajo"

	export := strings.Join([]string{
		"[15/05/2018, 10:30:00] " + group + ": Ana creó el grupo \"Amigos del trabajo\"",
		"[15/05/2018, 10:31:00] " + group + ": Ana te agregó",
		"[15/05/2018, 10:32:00] Ana: buenos días a todos",
		"[15/05/2018, 10:33:00] " + group + ": Tú saliste",
	}, "\n")

	res, err := Parse(strings.NewReader(export), Options{ChatName: group})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Messages) != 4 {
		t.Fatalf("%d messages, expected 4", len(res.Messages))
	}

	for _, i := range []int{0, 1, 3} {
		m := res.Messages[i]
		if m.Sender != nil {
			t.Errorf("message %d (%q): sender = %q, expected nil", i, m.Body, *m.Sender)
		}
		if m.Kind != "system" {
			t.Errorf("message %d (%q): kind = %q, expected system", i, m.Body, m.Kind)
		}
	}

	if m := res.Messages[2]; m.Sender == nil || *m.Sender != "Ana" || m.Kind != "text" {
		t.Errorf("Ana's real message was altered: %+v", m)
	}
	if !res.IsGroup {
		t.Error("IsGroup = false, expected true (the reclassified notices should feed the inference)")
	}
}

func TestReclassifyGroupNotice_it(t *testing.T) {
	const group = "Amici del lavoro"

	export := strings.Join([]string{
		"[15/05/2018, 10:30:00] " + group + ": Ana ha creato il gruppo \"Amici del lavoro\"",
		"[15/05/2018, 10:31:00] " + group + ": Ana ti ha aggiunto",
		"[15/05/2018, 10:32:00] Ana: buongiorno a tutti",
		"[15/05/2018, 10:33:00] " + group + ": Hai lasciato",
	}, "\n")

	res, err := Parse(strings.NewReader(export), Options{ChatName: group})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Messages) != 4 {
		t.Fatalf("%d messages, expected 4", len(res.Messages))
	}
	for _, i := range []int{0, 1, 3} {
		m := res.Messages[i]
		if m.Sender != nil || m.Kind != "system" {
			t.Errorf("message %d (%q): sender=%v kind=%q, expected nil/system", i, m.Body, m.Sender, m.Kind)
		}
	}
	if m := res.Messages[2]; m.Sender == nil || *m.Sender != "Ana" || m.Kind != "text" {
		t.Errorf("Ana's real message was altered: %+v", m)
	}
	if !res.IsGroup {
		t.Error("IsGroup = false, expected true")
	}
}

func TestReclassifyGroupNotice_fr(t *testing.T) {
	const group = "Amis du travail"

	export := strings.Join([]string{
		"[15/05/2018, 10:30:00] " + group + ": Ana a créé le groupe \"Amis du travail\"",
		"[15/05/2018, 10:31:00] " + group + ": Ana t'a ajouté",
		"[15/05/2018, 10:32:00] Ana: bonjour à tous",
		"[15/05/2018, 10:33:00] " + group + ": Tu as quitté",
	}, "\n")

	res, err := Parse(strings.NewReader(export), Options{ChatName: group})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Messages) != 4 {
		t.Fatalf("%d messages, expected 4", len(res.Messages))
	}
	for _, i := range []int{0, 1, 3} {
		m := res.Messages[i]
		if m.Sender != nil || m.Kind != "system" {
			t.Errorf("message %d (%q): sender=%v kind=%q, expected nil/system", i, m.Body, m.Sender, m.Kind)
		}
	}
	if m := res.Messages[2]; m.Sender == nil || *m.Sender != "Ana" || m.Kind != "text" {
		t.Errorf("Ana's real message was altered: %+v", m)
	}
	if !res.IsGroup {
		t.Error("IsGroup = false, expected true")
	}
}

func TestReclassifyGroupNotice_de(t *testing.T) {
	const group = "Arbeitsfreunde"

	export := strings.Join([]string{
		"[15/05/2018, 10:30:00] " + group + ": Ana hat die Gruppe erstellt",
		"[15/05/2018, 10:31:00] " + group + ": Ana hat dich hinzugefügt",
		"[15/05/2018, 10:32:00] Ana: guten Morgen zusammen",
		"[15/05/2018, 10:33:00] " + group + ": Du hast die Gruppe verlassen",
	}, "\n")

	res, err := Parse(strings.NewReader(export), Options{ChatName: group})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Messages) != 4 {
		t.Fatalf("%d messages, expected 4", len(res.Messages))
	}
	for _, i := range []int{0, 1, 3} {
		m := res.Messages[i]
		if m.Sender != nil || m.Kind != "system" {
			t.Errorf("message %d (%q): sender=%v kind=%q, expected nil/system", i, m.Body, m.Sender, m.Kind)
		}
	}
	if m := res.Messages[2]; m.Sender == nil || *m.Sender != "Ana" || m.Kind != "text" {
		t.Errorf("Ana's real message was altered: %+v", m)
	}
	if !res.IsGroup {
		t.Error("IsGroup = false, expected true")
	}
}

func TestReclassifyGroupNotice_nl(t *testing.T) {
	const group = "Werkvrienden"

	export := strings.Join([]string{
		"[15/05/2018, 10:30:00] " + group + ": Ana heeft de groep aangemaakt",
		"[15/05/2018, 10:31:00] " + group + ": Ana heeft je toegevoegd",
		"[15/05/2018, 10:32:00] Ana: goedemorgen allemaal",
		"[15/05/2018, 10:33:00] " + group + ": Je hebt de groep verlaten",
	}, "\n")

	res, err := Parse(strings.NewReader(export), Options{ChatName: group})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Messages) != 4 {
		t.Fatalf("%d messages, expected 4", len(res.Messages))
	}
	for _, i := range []int{0, 1, 3} {
		m := res.Messages[i]
		if m.Sender != nil || m.Kind != "system" {
			t.Errorf("message %d (%q): sender=%v kind=%q, expected nil/system", i, m.Body, m.Sender, m.Kind)
		}
	}
	if m := res.Messages[2]; m.Sender == nil || *m.Sender != "Ana" || m.Kind != "text" {
		t.Errorf("Ana's real message was altered: %+v", m)
	}
	if !res.IsGroup {
		t.Error("IsGroup = false, expected true")
	}
}

func TestReclassifyGroupNotice_leavesRealMessageAlone(t *testing.T) {
	cases := []struct {
		name     string
		chatName string
		line     string
	}{
		{
			"similar phrase coming from a person",
			"Reunião da treta",
			"[15/05/2018, 10:30:00] Ana: Você saiu",
		},
		{
			"group name, but normal conversation body",
			"Reunião da treta",
			"[15/05/2018, 10:30:00] Reunião da treta: alguém saiu mais cedo hoje?",
		},
		{
			"pattern word in the middle of a sentence",
			"Reunião da treta",
			"[15/05/2018, 10:30:00] Reunião da treta: ela adicionou açúcar demais no bolo",
		},
		{
			"1:1 conversation — the chat name is the contact and signs their messages",
			"Ana",
			"[15/05/2018, 10:30:00] Ana: saiu",
		},
		{
			"1:1 conversation with a phrase identical to a group notice",
			"Ana",
			"[15/05/2018, 10:30:00] Ana: Você saiu",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := Parse(strings.NewReader(c.line), Options{ChatName: c.chatName})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			m := res.Messages[0]
			if m.Sender == nil {
				t.Errorf("sender was wrongly erased in %q", m.Body)
			}
			if m.Kind != "text" {
				t.Errorf("kind = %q, expected text (%q)", m.Kind, m.Body)
			}
		})
	}
}

func TestReclassifyGroupNotice_withoutChatNameDoesNothing(t *testing.T) {
	line := "[15/05/2018, 10:30:00] Reunião da treta: Você saiu"
	res, err := Parse(strings.NewReader(line), Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if res.Messages[0].Sender == nil {
		t.Error("without ChatName nothing should be reclassified")
	}
}

func TestSameName(t *testing.T) {
	equal := [][2]string{
		{"Ana Souza", "Ana Souza"},
		{"ana souza", "Ana Souza"},
		{"  Ana Souza  ", "Ana Souza"},
		{"+55 21 99883-3621", "+55 21 99883‑3621"}, // non-breaking hyphen
		{"+55 21 99883-3621", "+55 21 99883–3621"}, // en dash
	}
	for _, c := range equal {
		if !SameName(c[0], c[1]) {
			t.Errorf("SameName(%q, %q) = false, expected true", c[0], c[1])
		}
	}

	different := [][2]string{
		{"Ana Souza", "Ana"},
		{"Karina Sassi 2025", "Karina Sassi"}, // different names
		{"Ana", ""},
	}
	for _, c := range different {
		if SameName(c[0], c[1]) {
			t.Errorf("SameName(%q, %q) = true, expected false", c[0], c[1])
		}
	}
}

func TestSecurityCodeNotice(t *testing.T) {
	export := strings.Join([]string{
		"26/07/2026 09:00 - Wallace: Seu código de segurança com Ana mudou.",
		"26/07/2026 09:01 - Wallace: e o código de segurança do banco eu te mando depois",
		"26/07/2026 09:02 - Ana: beleza",
	}, "\n")

	res, err := Parse(strings.NewReader(export), Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if res.Messages[0].Kind != "system" || res.Messages[0].Sender != nil {
		t.Errorf("the notice should become system with no sender, got kind=%q sender=%v",
			res.Messages[0].Kind, res.Messages[0].Sender)
	}
	if res.Messages[1].Kind != "text" || res.Messages[1].Sender == nil {
		t.Errorf("real message with 'código de segurança' was reclassified: kind=%q", res.Messages[1].Kind)
	}
}
