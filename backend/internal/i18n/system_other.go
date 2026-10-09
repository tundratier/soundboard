//go:build !windows

package i18n

import "os"

// System guesses the language from the locale environment variables (the app targets Windows; this keeps
// the package usable in tests and tools on other systems).
func System() Lang {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(name); v != "" {
			if l, ok := Parse(v); ok {
				return l
			}
			break
		}
	}
	return En
}
