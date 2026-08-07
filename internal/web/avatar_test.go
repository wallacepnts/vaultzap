package web

import "testing"

func TestAvatarClass_isDeterministic(t *testing.T) {
	if avatarClass("Vitor") != avatarClass("Vitor") {
		t.Error("classeAvatar deveria ser determinística para o mesmo nome")
	}
}

func TestAvatarClass_spreadsAcrossNames(t *testing.T) {
	names := []string{
		"Ana", "Bruno", "Carla", "Daniel", "Eduarda", "Fabio", "Gabriela",
		"Henrique", "Isabela", "João", "Karina", "Lucas", "Mariana", "Nicolas",
		"Olivia", "Pedro", "Quenia", "Rafael", "Sofia", "Thiago",
	}
	classes := map[string]bool{}
	for _, name := range names {
		classes[avatarClass(name)] = true
	}
	if len(classes) < 10 {
		t.Errorf("só %d classes distintas entre %d nomes, esperava bem mais variedade", len(classes), len(names))
	}
}

func TestIniciais(t *testing.T) {
	cases := map[string]string{
		"Vitor":         "V",
		"Vitor Hugo":    "VH",
		"  Ana   Lima ": "AL",
		"":              "?",
	}
	for input, want := range cases {
		if got := initials(input); got != want {
			t.Errorf("initials(%q) = %q, esperado %q", input, got, want)
		}
	}
}
