package i18n

import (
	"bytes"
	"log"
	"regexp"
	"strings"
	"testing"
)

func TestCatalogsAreComplete(t *testing.T) {
	for k := range en {
		if _, ok := de[k]; !ok {
			t.Errorf("key %q missing in the German catalog", k)
		}
	}
	for k, s := range de {
		if _, ok := en[k]; !ok {
			t.Errorf("key %q missing in the English catalog", k)
		}
		if strings.TrimSpace(s) == "" {
			t.Errorf("German text for %q is empty", k)
		}
	}
	for k, s := range en {
		if strings.TrimSpace(s) == "" {
			t.Errorf("English text for %q is empty", k)
		}
	}
}

var verbRe = regexp.MustCompile(`%[-+# 0]*[0-9]*(?:\.[0-9]+)?[a-zA-Z]`)

// Both languages must take the same arguments in the same order, otherwise one of them prints garbage.
func TestCatalogFormatVerbsMatch(t *testing.T) {
	for k, e := range en {
		if strings.HasPrefix(string(k), "flag.") {
			continue // plain descriptions, not format strings
		}
		ev, dv := verbRe.FindAllString(e, -1), verbRe.FindAllString(de[k], -1)
		if strings.Join(ev, " ") != strings.Join(dv, " ") {
			t.Errorf("%q: English verbs %v, German verbs %v", k, ev, dv)
		}
	}
}

func TestParseAndNormalize(t *testing.T) {
	for in, want := range map[string]Lang{"de": De, "DE": De, "de-DE": De, "de_AT": De, "en": En, "en-US": En, "en_GB": En} {
		if got, ok := Parse(in); !ok || got != want {
			t.Errorf("Parse(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "auto", "fr", "deutsch", "d"} {
		if _, ok := Parse(in); ok {
			t.Errorf("Parse(%q) should fail", in)
		}
	}
	for in, want := range map[string]string{"": "", "auto": "", " AUTO ": "", "de-DE": "de", "EN": "en"} {
		if got, ok := Normalize(in); !ok || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	if _, ok := Normalize("fr"); ok {
		t.Error("Normalize(fr) should fail")
	}
}

func TestResolveExplicitLanguageWins(t *testing.T) {
	if got := Resolve("de"); got != De {
		t.Errorf("Resolve(de) = %q", got)
	}
	if got := Resolve("en"); got != En {
		t.Errorf("Resolve(en) = %q", got)
	}
	if got := Resolve(SettingAuto); got != De && got != En {
		t.Errorf("Resolve(auto) = %q, want a supported language", got)
	}
}

func TestTUsesActiveLanguage(t *testing.T) {
	defer Set(Current())
	Set(De)
	if got := T(TrayQuit); got != "Beenden" {
		t.Errorf("German = %q", got)
	}
	Set(En)
	if got := T(TrayQuit); got != "Quit" {
		t.Errorf("English = %q", got)
	}
	if got := T(Key("no.such.key")); got != "no.such.key" {
		t.Errorf("unknown key = %q", got)
	}
}

func TestLogfWritesEnglishToFileAndActiveLanguageToConsole(t *testing.T) {
	defer Set(Current())
	defer SetLogSinks(nil, nil)
	var file, console bytes.Buffer
	SetLogSinks(log.New(&file, "", 0), log.New(&console, "", 0))
	Set(De)
	Logf(LogLibraryLoaded, 3, "1s")
	if got := strings.TrimSpace(file.String()); got != "library: loaded 3 sounds in 1s" {
		t.Errorf("file = %q", got)
	}
	if got := strings.TrimSpace(console.String()); got != "Bibliothek: 3 Sounds in 1s geladen" {
		t.Errorf("console = %q", got)
	}
}
