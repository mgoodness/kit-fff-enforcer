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
