package tools

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	RegisterTool("rollback", RollbackDefinition, RollbackFile)
}

// RollbackEntryReason describes why a rollback entry was (not) created.
const (
	RollbackCreated     = ""            // entry created, hash valid
	RollbackAbsent      = "absent"      // file did not exist (nothing to save)
	RollbackUnavailable = "unavailable" // file too large or read/write failed
)

// CreateRollbackEntry saves the pre-mutation state of the file at absPath
// to the .rollback directory. It stats and reads the file itself, so it is
// meant for tools that do not already hold the file content (delete,
// overwrite). Tools that have already read the file should use
// CreateRollbackEntryFromContent to avoid a redundant read and a TOCTOU gap
// between the read and the snapshot.
//
// The entry filename is the MD5 hash of (relative path + content), so
// identical states deduplicate to the same entry. Returns the hash and a
// reason string:
//   - ""            entry created, hash is valid
//   - "absent"      file did not exist, nothing saved
//   - "unavailable" file exceeds MaxRollbackSize or could not be read/written
func CreateRollbackEntry(absPath, relPath string) (hash, reason string) {
	_, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", RollbackAbsent
		}
		return "", RollbackUnavailable
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", RollbackUnavailable
	}
	return CreateRollbackEntryFromContent(relPath, content)
}

// CreateRollbackEntryFromContent saves the given pre-mutation content to a
// rollback entry for relPath. See CreateRollbackEntry for the return values.
func CreateRollbackEntryFromContent(relPath string, content []byte) (hash, reason string) {
	if len(content) > int(MaxRollbackSize.Load()) {
		return "", RollbackUnavailable
	}
	hash = fmt.Sprintf("%x", md5.Sum(append([]byte(relPath), append([]byte("\n"), content...)...)))
	if err := os.MkdirAll(RollbackDir, FileModeDirectory); err != nil {
		return "", RollbackUnavailable
	}
	header := fmt.Sprintf("%s %s %s %d\n", RollbackHeaderMarker, RollbackHeaderVersion, relPath, time.Now().Unix())
	entryPath := filepath.Join(RollbackDir, hash)
	if err := os.WriteFile(entryPath, append([]byte(header), content...), FileModeRegularFile); err != nil {
		return "", RollbackUnavailable
	}
	return hash, RollbackCreated
}

// relToWorkDir returns the working-directory-relative form of an absolute path.
func relToWorkDir(absPath string) (string, error) {
	baseDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(baseDir, absPath)
	if err != nil {
		return "", err
	}
	return rel, nil
}

// parseRollbackEntry splits an entry's raw content into (relPath, original content).
// The header occupies the first line: "<marker> <version> <relPath> <unix timestamp>".
// The timestamp is the last token; the relPath is everything between the
// fixed prefix and the last space, so paths containing spaces parse correctly.
func parseRollbackEntry(data []byte) (relPath string, content []byte, err error) {
	idx := strings.IndexByte(string(data), '\n')
	if idx == -1 {
		return "", nil, fmt.Errorf("malformed rollback entry: missing header")
	}
	header := string(data[:idx])
	content = data[idx+1:]

	prefix := RollbackHeaderMarker + " " + RollbackHeaderVersion + " "
	if !strings.HasPrefix(header, prefix) {
		return "", nil, fmt.Errorf("malformed rollback entry: bad header")
	}
	rest := strings.TrimPrefix(header, prefix)
	relPath, ts, ok := strings.CutLast(rest, " ")
	if !ok {
		return "", nil, fmt.Errorf("malformed rollback entry: bad header")
	}
	if relPath == "" || !isDigits(ts) {
		return "", nil, fmt.Errorf("malformed rollback entry: bad header")
	}
	return relPath, content, nil
}

// isDigits reports whether s is a non-empty string of digits.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// RollbackFile restores a file to its pre-mutation state identified by a
// rollback entry hash. If the file currently exists, its current state is
// saved as a new rollback entry first (chaining), so the restore itself is
// reversible. If the chain snapshot cannot be created (file > 100MB or
// write failure), the response reports "rollback": "unavailable" so the
// caller knows the restore was not made reversible.
func RollbackFile(args map[string]any) (any, error) {
	hash, ok := ArgString(args, "hash")
	if !ok || len(hash) != 32 {
		return ErrorResult("A valid rollback hash (32 hex characters) is required"), nil
	}
	for _, c := range hash {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return ErrorResult("A valid rollback hash (32 hex characters) is required"), nil
		}
	}

	entryPath := filepath.Join(RollbackDir, hash)
	data, err := os.ReadFile(entryPath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("no rollback entry found for hash '%s': %v", hash, err)), nil
	}

	relPath, content, err := parseRollbackEntry(data)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	absPath, err := ValidatePath(relPath)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	// Chain: save the current state before restoring, if the file exists.
	chainedHash, reason := CreateRollbackEntry(absPath, relPath)

	if err := writeFile(absPath, content); err != nil {
		return ErrorResult(err.Error()), nil
	}

	resp := map[string]any{
		"success": true,
		"path":    relPath,
	}
	switch reason {
	case RollbackCreated:
		resp["rollback"] = chainedHash
	case RollbackUnavailable:
		resp["rollback"] = "unavailable"
	}
	return resp, nil
}

// RollbackDefinition defines the tool schema.
var RollbackDefinition = map[string]any{
	"description": "Restore a file to its state before an edit, using a rollback entry hash.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"hash": map[string]any{"type": "string", "description": "Hash of the rollback entry"},
		},
		"required": []string{"hash"},
	},
}
