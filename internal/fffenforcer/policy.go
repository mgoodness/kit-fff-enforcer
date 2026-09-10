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
	"strings"
)

// shellGrepFind matches a bare `grep` or `find` invocation at the start of
// a shell command or after a `;`, `&&`, `&`, or `|` separator — i.e. as its
// own command, not as a substring of another word (like "sgrep" or
// "findutils") or inside a quoted argument to some other command.
var shellGrepFind = regexp.MustCompile(`(^|[;&|]\s*)(grep|find)\b`)

// networkFetch matches a command that pulls data over the network (curl,
// wget, an http(s) client) rather than reading files out of the repo.
var networkFetch = regexp.MustCompile(`^(curl|wget|https?)\b`)

// networkFetchSubstitution matches a process- or command-substitution whose
// body starts with a network fetch, e.g. the process substitution in
// `grep foo <(curl -s https://example.com)`, or the command substitution in
// `grep foo "$(curl -s https://example.com)"`. Grep filtering that kind of
// transient, non-repo output isn't a file search fff has anything indexed
// for, same as when a network fetch feeds grep through a plain pipe.
var networkFetchSubstitution = regexp.MustCompile(`[<$]\(\s*(curl|wget|https?)\b`)

// hardSeparator matches a shell separator that starts a fresh command
// chain -- `;` or `&` -- but deliberately not `|`, which chains pipeline
// stages together rather than resetting them. Used to find the start of
// the current pipe chain (or command substitution) without cutting it
// short at an intermediate stage.
var hardSeparator = regexp.MustCompile(`[;&]`)

// pipedFromNetworkFetch reports whether the `|` at index pipeIdx in cmd is
// part of a pipe chain whose leftmost command fetches data over the
// network rather than reading repo files -- e.g. the `curl ...` in
// `curl ... | grep foo`, or in `curl ... | jq ... | grep foo` (grep's
// immediate predecessor is jq, but the chain still originates at curl).
// Grep filtering that kind of transient, non-repo output isn't a file
// search fff has anything indexed for, so blocking it steers the model
// toward tools that can't do the job.
func pipedFromNetworkFetch(cmd string, pipeIdx int) bool {
	before := cmd[:pipeIdx]
	if locs := hardSeparator.FindAllStringIndex(before, -1); len(locs) > 0 {
		last := locs[len(locs)-1]
		before = before[last[1]:]
	}
	return networkFetch.MatchString(strings.TrimSpace(before))
}

// commandSegmentFrom returns the portion of cmd starting at index start and
// running up to (but not including) the next hard separator (`;` or `&`),
// or to the end of the string if there is none. Unlike a full top-level
// split, this deliberately does not stop at `|`, so a process substitution
// containing a pipe of its own stays intact for networkFetchSubstitution
// to match against.
func commandSegmentFrom(cmd string, start int) string {
	rest := cmd[start:]
	if loc := hardSeparator.FindStringIndex(rest); loc != nil {
		return rest[:loc[0]]
	}
	return rest
}

// grepFedByNetworkFetch reports whether the grep invocation starting at
// wordStart in cmd is fed data fff has nothing indexed for: either piped
// from a network fetch (directly, or through further filters like jq), or
// via a process/command substitution whose body is a network fetch. This
// intentionally does not apply to find: unlike grep, find doesn't consume
// piped/substituted stdin -- it walks whatever path it's given as an
// argument, which is still a real, fff-indexed filesystem search.
func grepFedByNetworkFetch(cmd string, sepStart, sepEnd, wordStart int) bool {
	if sepEnd > sepStart && cmd[sepStart] == '|' && pipedFromNetworkFetch(cmd, sepStart) {
		return true
	}
	return networkFetchSubstitution.MatchString(commandSegmentFrom(cmd, wordStart))
}

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
	for _, m := range shellGrepFind.FindAllStringSubmatchIndex(command, -1) {
		sepStart, sepEnd := m[2], m[3]
		word := command[m[4]:m[5]]
		if word == "grep" && grepFedByNetworkFetch(command, sepStart, sepEnd, m[4]) {
			continue // filtering network output, not repo files — nothing here for fff to have indexed
		}
		return Decision{
			Block:  true,
			Reason: fmt.Sprintf(reasonFmt, "fff__grep / fff__multi_grep / fff__find_files", "shell grep/find"),
		}
	}
	return allow
}
