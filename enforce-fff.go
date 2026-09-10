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
// is inlined rather than shared. See internal/fffenforcer/policy.go and
// policy_test.go in this repo for the same logic under ordinary
// `go test` — keep the two in sync when changing either one.
//
// This file must stay at the repository root: kit's `kit install`
// scanner only recognizes root-level *.go files (or main.go under an
// ext/*-ext/*-extensions/ subdirectory) as extensions, and skips
// internal/, cmd/, pkg/, test*/ entirely — which is also why the pure
// logic package lives under internal/ rather than at the root.
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

	// networkFetch matches a command that pulls data over the network
	// (curl, wget, an http(s) client) rather than reading files out of the
	// repo.
	networkFetch := regexp.MustCompile(`^(curl|wget|https?)\b`)

	// networkFetchSubstitution matches a process- or command-substitution
	// whose body starts with a network fetch, e.g. the process substitution
	// in `grep foo <(curl -s https://example.com)`, or the command
	// substitution in `grep foo "$(curl -s https://example.com)"`. Grep
	// filtering that kind of transient, non-repo output isn't a file search
	// fff has anything indexed for, same as when a network fetch feeds grep
	// through a plain pipe.
	networkFetchSubstitution := regexp.MustCompile(`[<$]\(\s*(curl|wget|https?)\b`)

	// hardSeparator matches a shell separator that starts a fresh command
	// chain -- `;` or `&` -- but deliberately not `|`, which chains pipeline
	// stages together rather than resetting them. Used to find the start of
	// the current pipe chain (or command substitution) without cutting it
	// short at an intermediate stage.
	hardSeparator := regexp.MustCompile(`[;&]`)

	// pipedFromNetworkFetch reports whether the `|` at index pipeIdx in cmd
	// is part of a pipe chain whose leftmost command fetches data over the
	// network rather than reading repo files -- e.g. the `curl ...` in
	// `curl ... | grep foo`, or in `curl ... | jq ... | grep foo` (grep's
	// immediate predecessor is jq, but the chain still originates at curl).
	// Grep filtering that kind of transient, non-repo output isn't a file
	// search fff has anything indexed for, so blocking it steers the model
	// toward tools that can't do the job.
	pipedFromNetworkFetch := func(cmd string, pipeIdx int) bool {
		before := cmd[:pipeIdx]
		if locs := hardSeparator.FindAllStringIndex(before, -1); len(locs) > 0 {
			last := locs[len(locs)-1]
			before = before[last[1]:]
		}
		return networkFetch.MatchString(strings.TrimSpace(before))
	}

	// commandSegmentFrom returns the portion of cmd starting at index start
	// and running up to (but not including) the next hard separator (`;` or
	// `&`), or to the end of the string if there is none. Unlike a full
	// top-level split, this deliberately does not stop at `|`, so a process
	// substitution containing a pipe of its own stays intact for
	// networkFetchSubstitution to match against.
	commandSegmentFrom := func(cmd string, start int) string {
		rest := cmd[start:]
		if loc := hardSeparator.FindStringIndex(rest); loc != nil {
			return rest[:loc[0]]
		}
		return rest
	}

	// grepFedByNetworkFetch reports whether the grep invocation starting at
	// wordStart in cmd is fed data fff has nothing indexed for: either piped
	// from a network fetch (directly, or through further filters like jq),
	// or via a process/command substitution whose body is a network fetch.
	// This intentionally does not apply to find: unlike grep, find doesn't
	// consume piped/substituted stdin -- it walks whatever path it's given
	// as an argument, which is still a real, fff-indexed filesystem search.
	grepFedByNetworkFetch := func(cmd string, sepStart, sepEnd, wordStart int) bool {
		if sepEnd > sepStart && cmd[sepStart] == '|' && pipedFromNetworkFetch(cmd, sepStart) {
			return true
		}
		return networkFetchSubstitution.MatchString(commandSegmentFrom(cmd, wordStart))
	}

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
			if !inGitWorkTree() {
				return nil
			}
			for _, m := range shellGrepFind.FindAllStringSubmatchIndex(input.Command, -1) {
				sepStart, sepEnd := m[2], m[3]
				word := input.Command[m[4]:m[5]]
				if word == "grep" && grepFedByNetworkFetch(input.Command, sepStart, sepEnd, m[4]) {
					continue // filtering network output, not repo files — nothing here for fff to have indexed
				}
				return block("fff__grep / fff__multi_grep / fff__find_files", "shell grep/find")
			}
		}
		return nil
	})
}
