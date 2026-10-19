package tools

import "time"

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

// MaxLoopHistory is the maximum number of entries to keep in loop detection history
const MaxLoopHistory = 5

// LogBufferSize is the maximum number of log lines to store in GlobalLog buffer
const LogBufferSize = 100

// SubagentLogLines is the maximum number of lines to store in the subagent
// monitoring buffer (shown in the GUI "Subagent" tab)
const SubagentLogLines = 100

// --- Timeouts and Delays ---

// ShellTimeLimit is the time limit for shell commands. If exceeded, the command
// continues in the background.
const ShellTimeLimit = 30 * time.Second

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

// MaxImageDimension is the maximum pixel dimension (width or height) for images sent to VLMs
const MaxImageDimension = 1024

// --- Image Processing ---

// MaxImageSize is the maximum image file size: 50MB
const MaxImageSize = 50 * 1024 * 1024

// KBConversion is the kilobyte conversion factor.
const KBConversion = 1024

// MBConversion is used for converting bytes to megabytes
const MBConversion = 1024 * 1024

// --- Limits ---

// MemoryHashLen is the number of hex characters used for memory hash IDs
const MemoryHashLen = 10

// MaxTextLimit is the maximum number of characters to allow for tool inputs and outputs
const MaxTextLimit = 100000

// DefaultMaxEntries is the default number of entries returned by listing tools
const DefaultMaxEntries = 100

// HardMaxEntries is the absolute maximum number of entries any listing tool can return
const HardMaxEntries = 500

// --- Rollback ---

// RollbackDir is the directory where pre-mutation file states are stored
const RollbackDir = ".rollback"

// MaxRollbackSize is the maximum file size for which a rollback entry is created: 100MB
const MaxRollbackSize = 100 * MBConversion

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
