package fffenforcer

import (
	"os/exec"
	"strings"
	"testing"
)

// TestExtensionLoads is a black-box smoke test: it shells out to the real
// kit binary (if available) and asks it to statically validate
// enforce-fff.go — i.e. load it into a Yaegi interpreter and confirm Init
// registers a handler. This exercises the actual yaegi-loadable file
// (which policy_test.go cannot reach, since kit's loader only accepts raw
// Go standard library + kit/ext, not this compiled module) without
// needing network access or an LLM call.
//
// Skipped when no "kit" binary is on PATH (e.g. a bare CI runner that
// hasn't installed it).
func TestExtensionLoads(t *testing.T) {
	kitPath, err := exec.LookPath("kit")
	if err != nil {
		t.Skip("kit binary not found on PATH; skipping extension-load smoke test")
	}

	out, err := exec.Command(kitPath, "extensions", "validate", "-e", "./enforce-fff.go").CombinedOutput()
	if err != nil {
		t.Fatalf("kit extensions validate failed: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, "enforce-fff.go") {
		t.Fatalf("expected output to mention enforce-fff.go, got:\n%s", got)
	}
	if strings.Contains(got, "0 handlers") {
		t.Fatalf("expected at least one registered handler, got:\n%s", got)
	}
}
