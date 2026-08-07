package parser

import (
	"strings"
	"testing"
)

const mark = "‎"

func checkCodes(res Result) []string {
	var codes []string
	for _, c := range res.Checks {
		codes = append(codes, c.Code)
	}
	return codes
}

// A clean export must produce no findings at all: an alarm on an untouched file is how a
// check teaches people to ignore it. The seconds-long inversions and the .vcf keeping its
// own name are both real WhatsApp behaviour, measured on a 7.5k-message export.
func TestCoherence_cleanExportIsSilent(t *testing.T) {
	texto := mark + "[19/10/2022, 00:23:47] Ana: oi\n" +
		"[19/10/2022, 00:23:46] Bruno: cheguei atrasado no relógio dele\n" +
		mark + "[19/10/2022, 00:24:00] Ana: " + mark + "<anexado: 00000042-PHOTO-2022-10-19-00-24-00.jpg>\n" +
		mark + "[19/10/2022, 00:25:00] Ana: <anexado: contrato.pdf>\n" +
		mark + "[19/10/2022, 00:26:00] Ana: <anexado: cartao.vcf>\n"

	res, err := Parse(strings.NewReader(texto), Options{ChatName: "Ana"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Checks) != 0 {
		t.Errorf("export limpo deveria ficar em silêncio, veio %+v", res.Checks)
	}
}

func TestCoherence_flagsPlantedMessage(t *testing.T) {
	texto := "[19/10/2022, 10:00:00] Ana: oi\n" +
		"[19/10/2022, 14:00:00] Bruno: tudo certo\n" +
		"[19/10/2022, 09:00:00] Ana: combinei de pagar na segunda\n"

	res, _ := Parse(strings.NewReader(texto), Options{ChatName: "Ana"})
	codes := checkCodes(res)
	if len(codes) != 1 || codes[0] != CheckOutOfOrder {
		t.Errorf("mensagem plantada horas antes deveria acusar %q, veio %v", CheckOutOfOrder, codes)
	}
}

// The mark is only evidence when the file shows it uses marks at all: an Android export has
// none, and treating that as suspicious would flag every Android export ever made.
func TestCoherence_flagsStrippedMark(t *testing.T) {
	comMarca := mark + "[19/10/2022, 10:00:00] Ana: " + mark + "<anexado: 00000001-PHOTO-2022-10-19-10-00-00.jpg>\n"
	semMarca := "[19/10/2022, 10:01:00] Ana: <anexado: 00000002-PHOTO-2022-10-19-10-01-00.jpg>\n"

	res, _ := Parse(strings.NewReader(comMarca+semMarca), Options{ChatName: "Ana"})
	if codes := checkCodes(res); len(codes) != 1 || codes[0] != CheckMarksMissing {
		t.Errorf("linha estrutural sem a marca deveria acusar %q, veio %v", CheckMarksMissing, codes)
	}

	semNenhuma := "[19/10/2022, 10:00:00] Ana: IMG-20221019-WA0001.jpg (arquivo anexado)\n" +
		"[19/10/2022, 10:01:00] Ana: IMG-20221019-WA0002.jpg (arquivo anexado)\n"
	res, _ = Parse(strings.NewReader(semNenhuma), Options{ChatName: "Ana"})
	if len(res.Checks) != 0 {
		t.Errorf("export sem marca nenhuma (Android) não pode acusar nada, veio %+v", res.Checks)
	}
}

func TestCoherence_flagsMediaOutOfConvention(t *testing.T) {
	texto := "[19/10/2022, 10:00:00] Ana: editado.jpg (arquivo anexado)\n" +
		"[19/10/2022, 10:01:00] Ana: IMG-20200101-WA0001.jpg (arquivo anexado)\n"

	res, _ := Parse(strings.NewReader(texto), Options{ChatName: "Ana"})
	codes := checkCodes(res)
	if len(codes) != 2 {
		t.Fatalf("esperava nome fora da convenção e data divergente, veio %v", codes)
	}
	if codes[0] != CheckMediaNaming || codes[1] != CheckMediaDate {
		t.Errorf("códigos = %v", codes)
	}
}

// marksByLine has to agree line for line with normalize, or a finding points at the wrong
// line of the file.
func TestMarksByLine_alignsWithNormalize(t *testing.T) {
	bruto := []byte("a\r\n" + mark + "b\nc\r" + mark + "d\n")
	linhas := strings.Split(strings.TrimSuffix(normalize(bruto), "\n"), "\n")
	marcas := marksByLine(bruto)

	if len(marcas) < len(linhas) {
		t.Fatalf("marcas=%d linhas=%d: índices não batem", len(marcas), len(linhas))
	}
	for i, esperado := range []bool{false, true, false, true} {
		if marcas[i] != esperado {
			t.Errorf("linha %d: marca=%v, esperado %v", i, marcas[i], esperado)
		}
	}
}
