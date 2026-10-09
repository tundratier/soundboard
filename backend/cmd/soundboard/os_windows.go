//go:build windows

package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"fyne.io/systray"

	"soundboard/internal/i18n"
	"soundboard/internal/winaudio"
)

//go:embed icon.ico
var iconICO []byte

// appName is the product name; it is not translated.
const appName = "Soundboard"

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	user32           = syscall.NewLazyDLL("user32.dll")
	procCreateMutex  = kernel32.NewProc("CreateMutexW")
	procMessageBoxW  = user32.NewProc("MessageBoxW")
	errAlreadyExists = syscall.Errno(183)
)

// acquireSingleInstance returns false if an instance is already running.
func acquireSingleInstance() bool {
	name, _ := syscall.UTF16PtrFromString("Local\\SoundboardSingleInstance")
	h, _, err := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return true // start anyway when in doubt
	}
	return err != errAlreadyExists // handle stays open until the process ends
}

// fatalBox shows an error dialog (the GUI exe has no console) and exits the program.
// dialogText is shown to the user in the active language; logText is English and goes to the log file.
func fatalBox(dialogText, logText string) {
	i18n.Print(fmt.Sprintf("FATAL: %s", logText), fmt.Sprintf(i18n.T(i18n.LogFatal), dialogText))
	t, _ := syscall.UTF16PtrFromString(appName)
	m, _ := syscall.UTF16PtrFromString(dialogText)
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x10) // MB_ICONERROR
	os.Exit(1)
}

func openBrowser(url string) {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		i18n.Logf(i18n.LogOpenBrowser, err)
	}
}

type trayConfig struct {
	AdminURL, BoardURL string
	Language           string               // current language setting: "" (automatic), "de" or "en"
	OnLanguage         func(setting string) // called when the user picks a language; must switch i18n itself
}

// runTray shows the tray icon and blocks until "Quit". onQuit cleans up.
// The texts come from i18n and are refreshed when the language is changed in the menu.
func runTray(cfg trayConfig, onQuit func()) {
	systray.Run(func() {
		systray.SetIcon(iconICO)
		systray.SetTitle(appName)
		systray.SetOnTapped(func() { openBrowser(cfg.AdminURL) })
		mAdmin := systray.AddMenuItem("", "")
		mBoard := systray.AddMenuItem("", "")
		systray.AddSeparator()
		mLang := systray.AddMenuItem("", "")
		type langItem struct {
			setting string
			item    *systray.MenuItem
		}
		langs := []langItem{
			{i18n.SettingAuto, mLang.AddSubMenuItemCheckbox("", "", false)},
			{"de", mLang.AddSubMenuItemCheckbox("Deutsch", "", false)},
			{"en", mLang.AddSubMenuItemCheckbox("English", "", false)},
		}
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("", "")

		setting := cfg.Language
		refresh := func() {
			systray.SetTooltip(i18n.T(i18n.TrayTooltip))
			mAdmin.SetTitle(i18n.T(i18n.TrayAdmin))
			mAdmin.SetTooltip(i18n.T(i18n.TrayAdminHint))
			mBoard.SetTitle(i18n.T(i18n.TrayBoards))
			mBoard.SetTooltip(i18n.T(i18n.TrayBoardsHint))
			mLang.SetTitle(i18n.T(i18n.TrayLanguage))
			mLang.SetTooltip(i18n.T(i18n.TrayLanguageHint))
			langs[0].item.SetTitle(i18n.T(i18n.TrayLangAuto)) // "Deutsch" and "English" are not translated
			mQuit.SetTitle(i18n.T(i18n.TrayQuit))
			for _, l := range langs {
				if l.setting == setting {
					l.item.Check()
				} else {
					l.item.Uncheck()
				}
			}
		}
		choose := func(s string) {
			if s != setting {
				setting = s
				cfg.OnLanguage(s)
			}
			refresh()
		}
		refresh()

		go func() {
			for {
				select {
				case <-mAdmin.ClickedCh:
					openBrowser(cfg.AdminURL)
				case <-mBoard.ClickedCh:
					openBrowser(cfg.BoardURL)
				case <-langs[0].item.ClickedCh:
					choose(langs[0].setting)
				case <-langs[1].item.ClickedCh:
					choose(langs[1].setting)
				case <-langs[2].item.ClickedCh:
					choose(langs[2].setting)
				case <-mQuit.ClickedCh:
					systray.Quit()
					return
				}
			}
		}()
	}, onQuit)
}

type windowsPlatform struct{}

func (windowsPlatform) SetDefaultDevice(deviceID, role string) error {
	if role == "all" {
		return winaudio.SetDefaultEndpoint(deviceID, winaudio.RoleConsole, winaudio.RoleMultimedia, winaudio.RoleCommunications)
	}
	return winaudio.SetDefaultEndpoint(deviceID, winaudio.RoleCommunications)
}
