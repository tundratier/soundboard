//go:build windows

package main

import (
	_ "embed"
	"log"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"fyne.io/systray"

	"soundboard/internal/winaudio"
)

//go:embed icon.ico
var iconICO []byte

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
// dialogText is shown to the user (see ui_de.go); logText goes to the log.
func fatalBox(dialogText, logText string) {
	log.Printf("FATAL: %s", logText)
	t, _ := syscall.UTF16PtrFromString(uiDialogTitle)
	m, _ := syscall.UTF16PtrFromString(dialogText)
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x10) // MB_ICONERROR
	os.Exit(1)
}

func openBrowser(url string) {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		log.Printf("opening browser: %v", err)
	}
}

// runTray shows the tray icon and blocks until "Quit". onQuit cleans up.
// The texts come from ui_de.go.
func runTray(adminURL, boardURL string, onQuit func()) {
	systray.Run(func() {
		systray.SetIcon(iconICO)
		systray.SetTitle(uiDialogTitle)
		systray.SetTooltip(uiTrayTooltip)
		systray.SetOnTapped(func() { openBrowser(adminURL) })
		mAdmin := systray.AddMenuItem(uiTrayAdmin, uiTrayAdminHint)
		mBoard := systray.AddMenuItem(uiTrayBoards, uiTrayBoardsHint)
		systray.AddSeparator()
		mQuit := systray.AddMenuItem(uiTrayQuit, "")
		go func() {
			for {
				select {
				case <-mAdmin.ClickedCh:
					openBrowser(adminURL)
				case <-mBoard.ClickedCh:
					openBrowser(boardURL)
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
