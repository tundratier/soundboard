package i18n

// Key identifies a message. Every key must exist in both catalogs (checked by a test); the English text of the
// log keys is also what the log file contains.
type Key string

const (
	// Error dialogs shown before the app can start. %v / %s are the underlying error.
	ErrDataFolder  Key = "err.dataFolder"
	ErrConfig      Key = "err.config"
	ErrSoundFolder Key = "err.soundFolder"
	ErrAudioSystem Key = "err.audioSystem"
	ErrWebServer   Key = "err.webServer"
	ErrBadLang     Key = "err.badLang" // %q is the rejected -lang value

	// Tray menu.
	TrayTooltip      Key = "tray.tooltip"
	TrayAdmin        Key = "tray.admin"
	TrayAdminHint    Key = "tray.adminHint"
	TrayBoards       Key = "tray.boards"
	TrayBoardsHint   Key = "tray.boardsHint"
	TrayLanguage     Key = "tray.language"
	TrayLanguageHint Key = "tray.languageHint"
	TrayLangAuto     Key = "tray.langAuto"
	TrayQuit         Key = "tray.quit"

	// Content created on the first start.
	ExampleSound Key = "example.sound" // file name of the example sound
	ExampleTile  Key = "example.tile"  // label of the example tile

	// Command-line flag descriptions (-h).
	FlagData      Key = "flag.data"
	FlagPort      Key = "flag.port"
	FlagNoBrowser Key = "flag.noBrowser"
	FlagVersion   Key = "flag.version"
	FlagLang      Key = "flag.lang"
	FlagUsage     Key = "flag.usage" // heading of the help output; %s is the program name

	// Log lines. English goes to the log file, the active language to the console.
	LogStart                Key = "log.start"
	LogAdminURL             Key = "log.adminURL"
	LogConfig               Key = "log.config"
	LogAlreadyRunning       Key = "log.alreadyRunning"
	LogLibraryLoaded        Key = "log.libraryLoaded"
	LogWatchUnavailable     Key = "log.watchUnavailable"
	LogServerError          Key = "log.serverError"
	LogExited               Key = "log.exited"
	LogSeed                 Key = "log.seed"
	LogAutoselect           Key = "log.autoselect"
	LogOpenBrowser          Key = "log.openBrowser"
	LogFatal                Key = "log.fatal"
	LogLanguage             Key = "log.language"
	LogListening            Key = "log.listening"
	LogUnknownTile          Key = "log.unknownTile"
	LogAudioOpenFailed      Key = "log.audioOpenFailed"
	LogAudioRunning         Key = "log.audioRunning"
	LogAudioInputOpenFailed Key = "log.audioInputOpenFailed"
	LogAudioInputRunning    Key = "log.audioInputRunning"
	LogLibraryReadFolder    Key = "log.libraryReadFolder"
	LogLibraryFile          Key = "log.libraryFile"
	LogLibraryWatcher       Key = "log.libraryWatcher"
)

var catalogs = map[Lang]map[Key]string{De: de, En: en}
