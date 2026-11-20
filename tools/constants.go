package tools

import (
	"sync/atomic"
	"time"
)

// init seeds the runtime limits with their defaults; main (and the Config
// dialog) overwrite them via atomic Store.
func init() {
	MaxLoopHistory.Store(DefaultMaxLoopHistory)
	ShellTimeLimitSeconds.Store(int64(DefaultShellTimeLimit.Seconds()))
	MaxImageDimension.Store(DefaultMaxImageDimension)
	MaxImageSize.Store(DefaultMaxImageSize)
	MaxTextLimit.Store(DefaultMaxTextLimit)
	ExaMaxCharacters.Store(DefaultExaMaxCharacters)
	RipgrepMaxResults.Store(DefaultRipgrepMaxResults)
	MaxRollbackSize.Store(DefaultMaxRollbackSize)
	MaxSleepSeconds.Store(DefaultMaxSleepSeconds)
}

// =============================================================================
// Constants extracted from magic numbers throughout the codebase
// These are exported so they can be used across packages including main
// =============================================================================

// --- Server Configuration ---

// DefaultServerPort is the default HTTP server port
const DefaultServerPort = 8001

// MaxPort is the maximum allowed port number
const MaxPort = 65536

// --- Loop Detection ---

// DefaultMaxLoopHistory is the default maximum number of entries to keep in
// loop detection history. Configurable via .skynet.yml.
const DefaultMaxLoopHistory = 5

// MaxLoopHistory is the maximum number of entries to keep in loop detection
// history. Defaults to DefaultMaxLoopHistory; updated at startup from the
// loaded settings and at runtime from the Config dialog. Atomic because it
// is read by tool handlers while the dialog may rewrite it.
var MaxLoopHistory atomic.Int64

// LogBufferSize is the maximum number of log lines to store in GlobalLog buffer
const LogBufferSize = 100

// SubagentLogLines is the maximum number of lines to store in the subagent
// monitoring buffer (shown in the GUI "Subagent" tab)
const SubagentLogLines = 100

// --- Timeouts and Delays ---

// DefaultShellTimeLimit is the default time limit for shell commands. If
// exceeded, the command continues in the background. Configurable via
// .skynet.yml.
const DefaultShellTimeLimit = 30 * time.Second

// ShellTimeLimitSeconds is the time limit for shell commands, in seconds.
// If exceeded, the command continues in the background. Defaults to
// DefaultShellTimeLimit; updated at startup and from the Config dialog.
// Atomic because it is read by tool handlers while the dialog may rewrite it.
var ShellTimeLimitSeconds atomic.Int64

// KillProcessGroupGracePeriodMs is the time to wait after SIGTERM before SIGKILL in killProcessGroup() (milliseconds)
const KillProcessGroupGracePeriodMs = 100

// ServerStartDelayMs is the brief delay when starting the server to ensure previous instance is stopped (milliseconds)
const ServerStartDelayMs = 100

// --- Text and Entry Limits ---

// SlugMaxLength is the maximum length for slug descriptions in memory files
const SlugMaxLength = 128

// DefaultNumLines is the default number of lines to read in read_from_line tool
const DefaultNumLines = 50

// JSONRPCVersion is the JSON-RPC protocol version string
const JSONRPCVersion = "2.0"

// JSONRPCErrInvalidParams is the JSON-RPC error code for invalid params/requests (-32603)
const JSONRPCErrInvalidParams = -32603

// --- UI Limits ---

// TUILogDisplayLines is the number of log lines to display in the TUI log window
const TUILogDisplayLines = 50

// --- File Permissions ---

// FileModeRegularFile is the permission mode for files (0644 octal)
const FileModeRegularFile = 0644

// FileModeDirectory is the permission mode for directories (0755 octal)
const FileModeDirectory = 0755

// --- Image Resolution Limit ---

// DefaultMaxImageDimension is the default maximum pixel dimension (width or
// height) for images sent to VLMs. Configurable via .skynet.yml.
const DefaultMaxImageDimension = 1024

// MaxImageDimension is the maximum pixel dimension (width or height) for
// images sent to VLMs. Defaults to DefaultMaxImageDimension; updated at
// startup and from the Config dialog. Atomic because it is read by tool
// handlers while the dialog may rewrite it.
var MaxImageDimension atomic.Int64

// --- Image Processing ---

// DefaultMaxImageSizeMB is the default maximum image file size in MB.
const DefaultMaxImageSizeMB = 50

// DefaultMaxImageSize is the default maximum image file size: 50MB.
const DefaultMaxImageSize = DefaultMaxImageSizeMB * MBConversion

// MaxImageSize is the maximum image file size. Defaults to
// DefaultMaxImageSize; updated at startup and from the Config dialog.
// Atomic because it is read by tool handlers while the dialog may rewrite it.
var MaxImageSize atomic.Int64

// KBConversion is the kilobyte conversion factor.
const KBConversion = 1024

// MBConversion is used for converting bytes to megabytes
const MBConversion = 1024 * 1024

// --- Limits ---

// MemoryHashLen is the number of hex characters used for memory hash IDs
const MemoryHashLen = 10

// DefaultMaxTextLimit is the default maximum number of characters to allow
// for tool inputs and outputs. Configurable via .skynet.yml.
const DefaultMaxTextLimit = 100000

// MaxTextLimit is the maximum number of characters to allow for tool inputs
// and outputs. Defaults to DefaultMaxTextLimit; updated at startup and from
// the Config dialog. Atomic because it is read by tool handlers while the
// dialog may rewrite it.
var MaxTextLimit atomic.Int64

// DefaultExaMaxCharacters is the default maximum number of characters that
// web_fetch_exa will request per page. Configurable via .skynet.yml.
const DefaultExaMaxCharacters = 100000

// ExaMaxCharacters is the maximum number of characters web_fetch_exa
// requests per page. Defaults to DefaultExaMaxCharacters; updated at
// startup and from the Config dialog. Atomic because it is read by tool
// handlers while the dialog may rewrite it.
var ExaMaxCharacters atomic.Int64

// DefaultMaxEntries is the default number of entries returned by listing tools
const DefaultMaxEntries = 100

// HardMaxEntries is the absolute maximum number of entries any listing tool can return
const HardMaxEntries = 500

// DefaultRipgrepMaxResults is the default maximum number of ripgrep matches to
// return when 'max_results' is not specified. Configurable via .skynet.yml.
const DefaultRipgrepMaxResults = 20

// RipgrepMaxResults is the default maximum number of ripgrep matches to return
// when 'max_results' is not specified. Defaults to DefaultRipgrepMaxResults;
// updated at startup and from the Config dialog. Atomic because it is read
// by tool handlers while the dialog may rewrite it.
var RipgrepMaxResults atomic.Int64

// DefaultRipgrepTimeout is the hard wall-clock limit for a single ripgrep
// search. A search exceeding it is killed and reported as a timeout.
const DefaultRipgrepTimeout = 30 * time.Second

// --- Rollback ---

// RollbackDir is the directory where pre-mutation file states are stored
const RollbackDir = ".rollback"

// DefaultMaxRollbackSizeMB is the default maximum rollback file size in MB.
const DefaultMaxRollbackSizeMB = 100

// DefaultMaxRollbackSize is the default maximum file size for which a rollback
// entry is created: 100MB.
const DefaultMaxRollbackSize = DefaultMaxRollbackSizeMB * MBConversion

// MaxRollbackSize is the maximum file size for which a rollback entry is
// created. Defaults to DefaultMaxRollbackSize; updated at startup and from
// the Config dialog. Atomic because it is read by tool handlers while the
// dialog may rewrite it.
var MaxRollbackSize atomic.Int64

// RollbackHeaderMarker is the first token of a rollback entry header line
const RollbackHeaderMarker = "SKNET-RB"

// RollbackHeaderVersion is the second token of a rollback entry header line
const RollbackHeaderVersion = "v1"

// --- Background Processes ---

// BackgroundIDLength is the number of hex characters in a background process id
const BackgroundIDLength = 8

// BackgroundMaxEntries is the maximum number of tracked background processes to keep
const BackgroundMaxEntries = 100

// BackgroundTerminateWait is how long to wait for a process to exit after terminate
const BackgroundTerminateWait = 2 * time.Second

// BackgroundTerminatePollInterval is how often to poll for exit after terminate
const BackgroundTerminatePollInterval = 50 * time.Millisecond
