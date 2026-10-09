//go:build windows

package i18n

import "syscall"

var procGetUserDefaultUILanguage = syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")

const langIDGerman = 0x07 // primary language ID of German (LANG_GERMAN)

// System returns German if the Windows display language is German (any region), English otherwise.
func System() Lang {
	id, _, _ := procGetUserDefaultUILanguage.Call()
	if id&0x3ff == langIDGerman {
		return De
	}
	return En
}
