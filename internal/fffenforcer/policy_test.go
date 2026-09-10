package fffenforcer

import (
	"strings"
	"testing"
)

func TestDecideCoreTool(t *testing.T) {
	cases := []struct {
		name          string
		toolName      string
		inGitWorkTree bool
		wantBlock     bool
		wantReasonSub string
	}{
		{
			name:          "grep blocked in git repo",
			toolName:      "grep",
			inGitWorkTree: true,
			wantBlock:     true,
			wantReasonSub: "fff__grep",
		},
		{
			name:          "find blocked in git repo",
			toolName:      "find",
			inGitWorkTree: true,
			wantBlock:     true,
			wantReasonSub: "fff__find_files",
		},
		{
			name:          "grep allowed outside git repo",
			toolName:      "grep",
			inGitWorkTree: false,
			wantBlock:     false,
		},
		{
			name:          "find allowed outside git repo",
			toolName:      "find",
			inGitWorkTree: false,
			wantBlock:     false,
		},
		{
			name:          "unrelated tool never blocked",
			toolName:      "read",
			inGitWorkTree: true,
			wantBlock:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecideCoreTool(tc.toolName, tc.inGitWorkTree)
			if got.Block != tc.wantBlock {
				t.Fatalf("Block = %v, want %v (reason: %q)", got.Block, tc.wantBlock, got.Reason)
			}
			if tc.wantBlock {
				if got.Reason == "" {
					t.Fatal("expected a non-empty Reason on block")
				}
				if !strings.Contains(got.Reason, tc.wantReasonSub) {
					t.Fatalf("Reason %q does not contain %q", got.Reason, tc.wantReasonSub)
				}
			} else if got.Reason != "" {
				t.Fatalf("expected empty Reason when not blocking, got %q", got.Reason)
			}
		})
	}
}

func TestDecideShellCommand(t *testing.T) {
	cases := []struct {
		name          string
		command       string
		inGitWorkTree bool
		wantBlock     bool
	}{
		{name: "bare grep blocked in git repo", command: "grep -r foo .", inGitWorkTree: true, wantBlock: true},
		{name: "bare find blocked in git repo", command: "find . -name foo", inGitWorkTree: true, wantBlock: true},
		{name: "grep after semicolon blocked", command: "echo hi; grep foo bar", inGitWorkTree: true, wantBlock: true},
		{name: "grep after && blocked", command: "cd /tmp && grep foo bar", inGitWorkTree: true, wantBlock: true},
		{name: "grep after pipe blocked", command: "cat file | grep foo", inGitWorkTree: true, wantBlock: true},
		{name: "find after ampersand blocked", command: "find . -name x & echo done", inGitWorkTree: true, wantBlock: true},
		{
			name:          "grep substring in another word not blocked",
			command:       "sgrep foo bar",
			inGitWorkTree: true,
			wantBlock:     false,
		},
		{
			name:          "find substring in another word not blocked",
			command:       "findutils --version",
			inGitWorkTree: true,
			wantBlock:     false,
		},
		{
			name:          "unrelated command not blocked",
			command:       "echo hi",
			inGitWorkTree: true,
			wantBlock:     false,
		},
		{
			name:          "grep allowed outside git repo",
			command:       "grep -r foo .",
			inGitWorkTree: false,
			wantBlock:     false,
		},
		{
			name:          "grep piped from curl not blocked",
			command:       "curl -s https://example.com | grep foo",
			inGitWorkTree: true,
			wantBlock:     false,
		},
		{
			name:          "grep piped from wget not blocked",
			command:       "wget -qO- https://example.com | grep foo",
			inGitWorkTree: true,
			wantBlock:     false,
		},
		{
			name:          "find piped from curl still blocked",
			command:       "curl -s https://example.com/list | find . -type f",
			inGitWorkTree: true,
			wantBlock:     true,
		},
		{
			name:          "grep piped from curl after other commands not blocked",
			command:       "echo start; curl -s https://example.com | grep foo",
			inGitWorkTree: true,
			wantBlock:     false,
		},
		{
			name:          "grep piped from cat still blocked",
			command:       "cat notes.txt | grep foo",
			inGitWorkTree: true,
			wantBlock:     true,
		},
		{
			name:          "grep piped from curl outside git repo not blocked",
			command:       "curl -s https://example.com | grep foo",
			inGitWorkTree: false,
			wantBlock:     false,
		},
		{
			name:          "grep piped from curl through jq (multi-stage) not blocked",
			command:       "curl -s https://example.com | jq .name | grep foo",
			inGitWorkTree: true,
			wantBlock:     false,
		},
		{
			name:          "grep piped from cat through jq (multi-stage) still blocked",
			command:       "cat notes.json | jq .name | grep foo",
			inGitWorkTree: true,
			wantBlock:     true,
		},
		{
			name:          "grep fed by process substitution of curl not blocked",
			command:       "grep foo <(curl -s https://example.com)",
			inGitWorkTree: true,
			wantBlock:     false,
		},
		{
			name:          "grep fed by command substitution of curl not blocked",
			command:       "grep foo \"$(curl -s https://example.com)\"",
			inGitWorkTree: true,
			wantBlock:     false,
		},
		{
			name:          "grep fed by process substitution of cat still blocked",
			command:       "grep foo <(cat notes.txt)",
			inGitWorkTree: true,
			wantBlock:     true,
		},
		{
			name:          "find fed by process substitution of curl still blocked",
			command:       "find <(curl -s https://example.com) -type f",
			inGitWorkTree: true,
			wantBlock:     true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecideShellCommand(tc.command, tc.inGitWorkTree)
			if got.Block != tc.wantBlock {
				t.Fatalf("command %q: Block = %v, want %v (reason: %q)", tc.command, got.Block, tc.wantBlock, got.Reason)
			}
			if tc.wantBlock && got.Reason == "" {
				t.Fatal("expected a non-empty Reason on block")
			}
		})
	}
}
