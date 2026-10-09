package main

// All German UI texts of the desktop shell (error dialogs, tray menu, example content) live in this file, so the
// program code itself stays English. Logs always use English text.

const (
	// Error dialogs shown before the app can start. %v is the underlying error.
	uiErrDataFolder   = "Datenordner nicht anlegbar: %v"
	uiErrConfig       = "Konfiguration nicht lesbar: %v"
	uiErrSoundFolder  = "Sound-Ordner: %v"
	uiErrAudioSystem  = "Audio-System nicht verfügbar: %v"
	uiErrWebServer    = "Webserver konnte nicht starten:\n%v\n\nAnderer Port: soundboard.exe -port 9000"
	uiDialogTitle     = "Soundboard"
	uiTrayTooltip     = "Soundboard - Klick öffnet die Verwaltung"
	uiTrayAdmin       = "Verwaltung öffnen"
	uiTrayAdminHint   = "Geräte und Board konfigurieren"
	uiTrayBoards      = "Boards öffnen"
	uiTrayBoardsHint  = "Board-Übersicht im Browser"
	uiTrayQuit        = "Beenden"
	uiExampleSound    = "Testton.wav" // created on first start
	uiExampleTileName = "Testton"
)
