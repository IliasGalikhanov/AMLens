package i18n

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLanguageNegotiation(t *testing.T) {
	for header, want := range map[string]string{"": "en", "de": "en", "kk-KZ": "kk", "RU-ru": "ru", "ru;q=0.2, kk-KZ;q=0.9, en;q=0.5": "kk", "ru;q=0,en;q=0.7": "en", "ru;q=invalid": "en"} {
		if got := Parse(header); got != want {
			t.Fatalf("%s: %s != %s", header, got, want)
		}
	}
}
func TestEveryMessageTemplate(t *testing.T) {
	sample := func(template string) string {
		return placeholders.ReplaceAllStringFunc(template, func(token string) string {
			switch token {
			case "%%":
				return "%"
			case "%d":
				return "9007199254741001"
			case "%t":
				return "true"
			case "%s", "%w":
				return "example"
			default:
				return "123.4"
			}
		})
	}
	for source, entry := range catalog {
		for locale, translation := range map[string]string{"en": entry.EN, "kk": entry.KK} {
			if got, want := Text(sample(source), locale), sample(translation); got != want {
				t.Errorf("%s %q: got %q, want %q", locale, source, got, want)
			}
		}
	}
}
func TestCompositeAndNestedErrors(t *testing.T) {
	source := "Признаки транзита: входов=2, выходов=3, out/in=98.1%; полный баланс неизвестен"
	want := "Indicators of transit: incoming=2, outgoing=3, out/in=98.1%; full account balance is unknown"
	if got := Text(source, "en"); got != want {
		t.Fatal(got)
	}
	source = "nodes.parquet: строка 2, поле gid: повторный идентификатор"
	if got := Text(source, "en"); got != "nodes.parquet: row 2, field gid: duplicate identifier" {
		t.Fatal(got)
	}
	if got := Text(source, "kk"); !strings.Contains(got, "қайталанған идентификатор") {
		t.Fatal(got)
	}
	if Text(source, "ru") != source {
		t.Fatal("Russian source changed")
	}
}
func TestJSONPreservesNumbersIdentifiersAndUserText(t *testing.T) {
	source := []byte(`{"gid":"9007199254741001","number":9007199254741001,"amount":0.1234567890123456789,"message":"Маршрут не найден","answer":"Маршрут не найден","limitations":["В схеме нет остатков"],"role":"transit"}`)
	data, err := JSON(source, "en")
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]json.RawMessage
	if err = json.Unmarshal(data, &values); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"gid": `"9007199254741001"`, "number": "9007199254741001", "amount": "0.1234567890123456789", "message": `"Route not found"`, "answer": `"Маршрут не найден"`, "role": `"transit"`} {
		if string(values[key]) != want {
			t.Fatalf("%s: %s", key, values[key])
		}
	}
	if string(values["limitations"]) != `["The schema contains no account balances"]` {
		t.Fatal(string(data))
	}
}
func TestCSVTranslatesOnlyNarrativeColumns(t *testing.T) {
	data, err := CSV([]byte("gid,role,evidence\n9007199254741001,transit,В схеме нет остатков\n"), "en")
	if err != nil || string(data) != "gid,role,evidence\n9007199254741001,transit,The schema contains no account balances\n" {
		t.Fatal(string(data), err)
	}
}
