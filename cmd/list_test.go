package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/stretchr/testify/assert"
)

func TestGetDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		absPath  string
		wtPrefix string
		want     string
	}{
		{
			name:     "standard worktree with prefix",
			absPath:  "/workspace/wt-add-auth",
			wtPrefix: "wt-",
			want:     "add-auth",
		},
		{
			name:     "main worktree without prefix",
			absPath:  "/workspace/main",
			wtPrefix: "wt-",
			want:     "[main]",
		},
		{
			name:     "different prefix",
			absPath:  "/workspace/work-feature",
			wtPrefix: "work-",
			want:     "feature",
		},
		{
			name:     "empty prefix matches everything",
			absPath:  "/workspace/anything",
			wtPrefix: "",
			want:     "anything",
		},
		{
			name:     "partial prefix match wraps in brackets",
			absPath:  "/workspace/wt_add-auth",
			wtPrefix: "wt-",
			want:     "[wt_add-auth]",
		},
		{
			name:     "nested path extracts basename",
			absPath:  "/deep/nested/path/wt-feature",
			wtPrefix: "wt-",
			want:     "feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getDisplayName(tt.absPath, tt.wtPrefix)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShortSHASafe(t *testing.T) {
	tests := []struct {
		name   string
		sha    string
		maxLen int
		want   string
	}{
		{
			name:   "normal SHA truncated",
			sha:    "abc1234def5678",
			maxLen: 7,
			want:   "abc1234",
		},
		{
			name:   "SHA exactly maxLen",
			sha:    "abc1234",
			maxLen: 7,
			want:   "abc1234",
		},
		{
			name:   "SHA shorter than maxLen",
			sha:    "abc",
			maxLen: 7,
			want:   "abc",
		},
		{
			name:   "empty SHA returns placeholder",
			sha:    "",
			maxLen: 7,
			want:   "(no sha)",
		},
		{
			name:   "maxLen of 0 returns empty",
			sha:    "abc1234",
			maxLen: 0,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortSHASafe(tt.sha, tt.maxLen)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatWorktree(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		worktree    git.Worktree
		wtPrefix    string
		wantPath    string
		wantDisplay string
	}{
		{
			name: "local branch with prefix stripped",
			worktree: git.Worktree{
				AbsolutePath: "/ws/wt-add-auth",
				Ref: git.NewLocalBranch(
					"feature/add-auth",
					"origin/feature/add-auth",
					"/ws/wt-add-auth",
					true,
					0, 0,
					git.NewCommit("abc1234def5678", "Add auth", now, "user"),
				),
			},
			wtPrefix:    "wt-",
			wantPath:    "/ws/wt-add-auth",
			wantDisplay: "local branch add-auth feature/add-auth",
		},
		{
			name: "local branch without prefix match",
			worktree: git.Worktree{
				AbsolutePath: "/ws/main",
				Ref: git.NewLocalBranch(
					"main",
					"origin/main",
					"/ws/main",
					true,
					0, 0,
					git.NewCommit("abc1234def5678", "Initial", now, "user"),
				),
			},
			wtPrefix:    "wt-",
			wantPath:    "/ws/main",
			wantDisplay: "local branch [main] main",
		},
		{
			name: "tag worktree",
			worktree: git.Worktree{
				AbsolutePath: "/ws/wt-v1",
				Ref: git.NewTag(
					"v1.0.0",
					git.NewCommit("abc1234def5678", "Release", now, "user"),
					"Release v1.0.0",
					"Tagger",
					"tagger@example.com",
					now,
				),
			},
			wtPrefix:    "wt-",
			wantPath:    "/ws/wt-v1",
			wantDisplay: "tag v1 v1.0.0",
		},
		{
			name: "detached HEAD worktree",
			worktree: git.Worktree{
				AbsolutePath: "/ws/wt-hotfix",
				Ref:          git.NewCommit("abc1234def5678", "Hotfix", now, "user"),
			},
			wtPrefix:    "wt-",
			wantPath:    "/ws/wt-hotfix",
			wantDisplay: "detached hotfix abc1234",
		},
		{
			name: "detached HEAD with short SHA",
			worktree: git.Worktree{
				AbsolutePath: "/ws/wt-short",
				Ref:          git.NewCommit("abc", "Short SHA", now, "user"),
			},
			wtPrefix:    "wt-",
			wantPath:    "/ws/wt-short",
			wantDisplay: "detached short abc",
		},
		{
			name: "detached HEAD with empty SHA",
			worktree: git.Worktree{
				AbsolutePath: "/ws/wt-empty",
				Ref:          git.NewCommit("", "No SHA", now, "user"),
			},
			wtPrefix:    "wt-",
			wantPath:    "/ws/wt-empty",
			wantDisplay: "detached empty (no sha)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotDisplay := formatWorktree(tt.worktree, tt.wtPrefix)
			assert.Equal(t, tt.wantPath, gotPath)
			assert.Equal(t, tt.wantDisplay, gotDisplay)
		})
	}
}

func TestFormatWorktreeTabSeparation(t *testing.T) {
	now := time.Now()

	worktree := git.Worktree{
		AbsolutePath: "/ws/wt-test",
		Ref: git.NewLocalBranch(
			"feature/test",
			"",
			"/ws/wt-test",
			true,
			0, 0,
			git.NewCommit("abc1234", "Test", now, "user"),
		),
	}

	_, display := formatWorktree(worktree, "wt-")

	// Verify no tabs in display string
	assert.NotContains(t, display, "\t", "display string should not contain tabs")

	// Verify no trailing whitespace
	assert.Equal(t, display, strings.TrimRight(display, " \t"), "display string should have no trailing whitespace")

	// Verify proper spacing (single spaces between parts)
	assert.NotContains(t, display, "  ", "display string should not have double spaces")
}
