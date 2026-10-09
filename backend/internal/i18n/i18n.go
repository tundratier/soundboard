// Package i18n holds the language of the desktop shell and the backend: the tray menu, error dialogs, flag help,
// console output and the first-run example content. The web frontend has its own catalogs (frontend/src/i18n) and
// asks the server which language to use.
//
// Two languages exist, German and English. The language is chosen once at start from the -lang flag or the saved
// setting; "auto" (the default) means German if the Windows display language is German, English otherwise.
package i18n

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

// Lang is a supported UI language.
type Lang string

const (
	De Lang = "de"
	En Lang = "en"
)

// SettingAuto is the stored setting for "follow the system language"; the other settings are "de" and "en".
const SettingAuto = ""

var (
	mu      sync.RWMutex
	current = En

	// Log sinks, see Print. Both nil: fall back to the standard logger.
	fileLog, consoleLog *log.Logger
)

// Parse understands "de", "en" and region variants such as "de-DE" or "en_US" (case-insensitive).
func Parse(s string) (Lang, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case s == "de" || strings.HasPrefix(s, "de-") || strings.HasPrefix(s, "de_"):
		return De, true
	case s == "en" || strings.HasPrefix(s, "en-") || strings.HasPrefix(s, "en_"):
		return En, true
	}
	return "", false
}

// Normalize maps user input to a stored setting: "" / "auto" → SettingAuto, a language → "de" or "en".
// ok is false for anything else.
func Normalize(s string) (setting string, ok bool) {
	if t := strings.ToLower(strings.TrimSpace(s)); t == "" || t == "auto" {
		return SettingAuto, true
	}
	l, ok := Parse(s)
	return string(l), ok
}

// Resolve turns a setting into the language to use; automatic (or an unknown value) follows the system language.
func Resolve(setting string) Lang {
	if l, ok := Parse(setting); ok {
		return l
	}
	return System()
}

// Set switches the active language.
func Set(l Lang) {
	mu.Lock()
	current = l
	mu.Unlock()
}

// Current returns the active language.
func Current() Lang {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// T returns the text for k in the active language (English if the key is missing there, the key itself as a last
// resort, so a typo is visible instead of blank).
func T(k Key) string {
	if s, ok := lookup(Current(), k); ok {
		return s
	}
	return string(k)
}

func lookup(l Lang, k Key) (string, bool) {
	if s, ok := catalogs[l][k]; ok {
		return s, true
	}
	s, ok := catalogs[En][k]
	return s, ok
}

// SetLogSinks directs Print/Logf output: file gets the English text (stable for bug reports), console the text in
// the active language. Either may be nil.
func SetLogSinks(file, console *log.Logger) {
	mu.Lock()
	fileLog, consoleLog = file, console
	mu.Unlock()
}

// Print writes one log line: english to the log file, local to the console. Without sinks (tests, tools) the
// English text goes to the standard logger.
func Print(english, local string) {
	mu.RLock()
	f, c := fileLog, consoleLog
	mu.RUnlock()
	if f == nil && c == nil {
		log.Print(english)
		return
	}
	if f != nil {
		f.Print(english)
	}
	if c != nil {
		c.Print(local)
	}
}

// Logf logs a catalog message: English to the log file, the active language to the console.
func Logf(k Key, args ...any) {
	en, _ := lookup(En, k)
	local, ok := lookup(Current(), k)
	if !ok {
		local = en
	}
	Print(fmt.Sprintf(en, args...), fmt.Sprintf(local, args...))
}
