package testsupport_test

import (
	"bytes"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLocalCalendarKeyGeneratorProducesCanonicalCredentialKey(testingContext *testing.T) {
	keyPath := filepath.Join(testingContext.TempDir(), "calendar-key")
	output, generateError := calendarKeyCommand(testingContext, "generate", keyPath).CombinedOutput()
	if generateError != nil {
		testingContext.Fatalf("generate calendar key: %v; output = %s", generateError, output)
	}
	keyBytes, readError := os.ReadFile(keyPath)
	if readError != nil {
		testingContext.Fatalf("read generated calendar key: %v", readError)
	}
	decodedKey, decodeError := base64.StdEncoding.DecodeString(strings.TrimSpace(string(keyBytes)))
	if decodeError != nil || len(decodedKey) != 32 {
		testingContext.Fatalf("decoded calendar key bytes = %d, error = %v", len(decodedKey), decodeError)
	}
	fileInfo, statError := os.Stat(keyPath)
	if statError != nil {
		testingContext.Fatalf("inspect generated calendar key: %v", statError)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		testingContext.Fatalf("calendar key mode = %04o, want 0600", fileInfo.Mode().Perm())
	}
	if output, validationError := calendarKeyCommand(testingContext, "validate", keyPath).CombinedOutput(); validationError != nil {
		testingContext.Fatalf("validate generated calendar key: %v; output = %s", validationError, output)
	}
}

func TestLocalCalendarKeyValidatorRejectsHexEncoding(testingContext *testing.T) {
	keyPath := filepath.Join(testingContext.TempDir(), "calendar-key")
	if writeError := os.WriteFile(keyPath, []byte(strings.Repeat("ab", 32)+"\n"), 0o600); writeError != nil {
		testingContext.Fatalf("write hex calendar key: %v", writeError)
	}
	output, validationError := calendarKeyCommand(testingContext, "validate", keyPath).CombinedOutput()
	if validationError == nil {
		testingContext.Fatal("hex calendar key passed canonical validation")
	}
	if !bytes.Contains(output, []byte("32 base64-encoded bytes")) {
		testingContext.Fatalf("validation output = %q", output)
	}
}

func calendarKeyCommand(testingContext *testing.T, arguments ...string) *exec.Cmd {
	testingContext.Helper()
	_, filename, _, callerFound := runtime.Caller(0)
	if !callerFound {
		testingContext.Fatal("resolve calendar key test path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	return exec.Command(filepath.Join(repositoryRoot, "scripts", "calendar-key.sh"), arguments...)
}
