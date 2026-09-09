// Package fffenforcer contains the pure decision logic behind the
// enforce-fff kit extension: which tool calls get blocked, and why.
//
// This logic is duplicated (not imported) into ../../enforce-fff.go, the
// actual kit extension entry point. Kit's Yaegi loader evaluates that
// file's raw source text in an interpreter that only exposes the Go
// standard library and kit's own "kit/ext" API — it cannot import this
// compiled package, or any other third-party or local Go package. Keeping
// the decision logic here, under an ordinary `go test`, is what makes it
// testable at all; see ../../enforce-fff.go for the thin, yaegi-loadable
// wrapper that calls into the same logic inline.
//
// This package lives under internal/ specifically so kit's own
// `kit install` extension scanner skips it: the scanner treats every
// root-level *.go file as a candidate extension, and a file with no
// Init(api ext.API) function would fail to load (silently, but on every
// kit startup) if it were scanned.
package fffenforcer

import (
	"fmt"
	"regexp"
)

// shellGrepFind matches a bare `grep` or `find` invocation at the start of
// a shell command or after a `;`, `&&`, `&`, or `|` separator — i.e. as its
// own command, not as a substring of another word (like "sgrep" or
// "findutils") or inside a quoted argument to some other command.
var shellGrepFind = regexp.MustCompile(`(^|[;&|]\s*)(grep|find)\b`)

// Decision describes the outcome of evaluating a tool call.
type Decision struct {
	Block  bool
	Reason string
}

const reasonFmt = "Blocked: use the fff MCP tools (%s) instead of the built-in %s in this git-indexed directory."

// allow is the zero Decision: do not block.
var allow = Decision{}

// DecideCoreTool evaluates a call to one of kit's built-in "grep" or "find"
// core tools. inGitWorkTree reports whether the current directory is
// inside a git work tree (fff has nothing indexed outside one, so there is
// nothing to steer the model toward).
func DecideCoreTool(toolName string, inGitWorkTree bool) Decision {
	if !inGitWorkTree {
		return allow
	}
	switch toolName {
	case "grep":
		return Decision{Block: true, Reason: fmt.Sprintf(reasonFmt, "fff__grep / fff__multi_grep", "grep tool")}
	case "find":
		return Decision{Block: true, Reason: fmt.Sprintf(reasonFmt, "fff__find_files", "find tool")}
	default:
		return allow
	}
}

// DecideShellCommand evaluates a call to kit's built-in "shell" tool,
// given the command string it was asked to run. inGitWorkTree reports
// whether the current directory is inside a git work tree.
func DecideShellCommand(command string, inGitWorkTree bool) Decision {
	if !inGitWorkTree {
		return allow
	}
	if !shellGrepFind.MatchString(command) {
		return allow
	}
	return Decision{
		Block:  true,
		Reason: fmt.Sprintf(reasonFmt, "fff__grep / fff__multi_grep / fff__find_files", "shell grep/find"),
	}
}
