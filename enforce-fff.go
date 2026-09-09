//go:build ignore

package main

// enforce-fff.go — a kit extension that blocks the built-in "grep"/"find"
// core tools, and shell invocations of the grep/find binaries, whenever
// the current directory is a git work tree — steering the model to the
// fff MCP tools (fff__grep, fff__multi_grep, fff__find_files) instead.
// Outside a git repo (where fff has nothing indexed) grep/find/shell are
// left alone.
//
// kit's Yaegi loader evaluates this file's raw source text in an
// interpreter that only exposes the Go standard library and kit's own
// "kit/ext" API — it cannot import github.com/mgoodness/kit-fff-enforcer
// or any other local/third-party Go package, so the decision logic below
// is inlined rather than shared. See policy.go / policy_test.go in this
// repo for the same logic under ordinary `go test` — keep the two in
// sync when changing either one.
//
// Install: kit install github.com/mgoodness/kit-fff-enforcer
// Or drop this file directly into ~/.config/kit/extensions/.

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"kit/ext"
)

func Init(api ext.API) {
	shellGrepFind := regexp.MustCompile(`(^|[;&|]\s*)(grep|find)\b`)

	inGitWorkTree := func() bool {
		out, err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Output()
		return err == nil && strings.TrimSpace(string(out)) == "true"
	}

	block := func(replacement, blocked string) *ext.ToolCallResult {
		return &ext.ToolCallResult{
			Block: true,
			Reason: fmt.Sprintf(
				"Blocked: use the fff MCP tools (%s) instead of the built-in %s in this git-indexed directory.",
				replacement, blocked,
			),
		}
	}

	api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		switch tc.ToolName {
		case "grep":
			if inGitWorkTree() {
				return block("fff__grep / fff__multi_grep", "grep tool")
			}
		case "find":
			if inGitWorkTree() {
				return block("fff__find_files", "find tool")
			}
		case "shell":
			var input struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal([]byte(tc.Input), &input); err != nil {
				return nil
			}
			if shellGrepFind.MatchString(input.Command) && inGitWorkTree() {
				return block("fff__grep / fff__multi_grep / fff__find_files", "shell grep/find")
			}
		}
		return nil
	})
}
