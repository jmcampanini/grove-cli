package naming

import (
	"testing"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestWorktreeNameGenerator_Generate(t *testing.T) {
	tests := []struct {
		name        string
		worktreeCfg config.WorktreeConfig
		slugifyCfg  config.SlugifyConfig
		branchName  string
		want        string
	}{
		{
			name: "strip feature prefix",
			worktreeCfg: config.WorktreeConfig{
				NewPrefix:         "wt-",
				StripBranchPrefix: []string{"feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/add-user-auth",
			want:       "wt-add-user-auth",
		},
		{
			name: "strip fix prefix",
			worktreeCfg: config.WorktreeConfig{
				NewPrefix:         "wt-",
				StripBranchPrefix: []string{"feature/", "fix/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "fix/login-bug",
			want:       "wt-login-bug",
		},
		{
			name: "first matching prefix stripped",
			worktreeCfg: config.WorktreeConfig{
				NewPrefix:         "wt-",
				StripBranchPrefix: []string{"fix/", "feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/add-auth",
			want:       "wt-add-auth",
		},
		{
			name: "no matching prefix",
			worktreeCfg: config.WorktreeConfig{
				NewPrefix:         "wt-",
				StripBranchPrefix: []string{"feature/", "fix/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "chore/update-deps",
			want:       "wt-chore-update-deps",
		},
		{
			name: "empty strip prefix list",
			worktreeCfg: config.WorktreeConfig{
				NewPrefix:         "wt-",
				StripBranchPrefix: []string{},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/add-auth",
			want:       "wt-feature-add-auth",
		},
		{
			name: "different worktree prefix",
			worktreeCfg: config.WorktreeConfig{
				NewPrefix:         "work-",
				StripBranchPrefix: []string{"feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/my-feature",
			want:       "work-my-feature",
		},
		{
			name: "empty worktree prefix",
			worktreeCfg: config.WorktreeConfig{
				NewPrefix:         "",
				StripBranchPrefix: []string{"feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/add-auth",
			want:       "add-auth",
		},
		{
			name: "empty branch name returns empty",
			worktreeCfg: config.WorktreeConfig{
				NewPrefix:         "wt-",
				StripBranchPrefix: []string{"feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "",
			want:       "",
		},
		{
			name: "branch name with uppercase gets lowercased",
			worktreeCfg: config.WorktreeConfig{
				NewPrefix:         "wt-",
				StripBranchPrefix: []string{"feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/ADD-USER-AUTH",
			want:       "wt-add-user-auth",
		},
		{
			name: "branch name with special chars gets slugified",
			worktreeCfg: config.WorktreeConfig{
				NewPrefix:         "wt-",
				StripBranchPrefix: []string{"feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/add@user#auth",
			want:       "wt-add-user-auth",
		},
		{
			name: "branch name only has prefix returns empty",
			worktreeCfg: config.WorktreeConfig{
				NewPrefix:         "wt-",
				StripBranchPrefix: []string{"feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/",
			want:       "",
		},
		{
			name: "main branch without prefix",
			worktreeCfg: config.WorktreeConfig{
				NewPrefix:         "wt-",
				StripBranchPrefix: []string{"feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "main",
			want:       "wt-main",
		},
		{
			name: "nested prefix pattern",
			worktreeCfg: config.WorktreeConfig{
				NewPrefix:         "wt-",
				StripBranchPrefix: []string{"feature/jcamp/", "feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/jcamp/add-auth",
			want:       "wt-add-auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewWorktreeNameGenerator(tt.worktreeCfg, tt.slugifyCfg)
			got := gen.Generate(tt.branchName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewWorktreeNameGenerator(t *testing.T) {
	worktreeCfg := config.WorktreeConfig{
		NewPrefix:         "test-",
		StripBranchPrefix: []string{"a/", "b/"},
	}
	slugCfg := config.SlugifyConfig{
		CollapseDashes:     false,
		HashLength:         8,
		Lowercase:          false,
		MaxLength:          75,
		ReplaceNonAlphanum: false,
		TrimDashes:         false,
	}

	gen := NewWorktreeNameGenerator(worktreeCfg, slugCfg)

	assert.Equal(t, "test-", gen.prefix)
	assert.Equal(t, []string{"a/", "b/"}, gen.stripBranchPrefix)
	assert.Equal(t, 8, gen.slugifyOpts.HashLength)
	assert.False(t, gen.slugifyOpts.Lowercase)
	assert.Equal(t, 75, gen.slugifyOpts.MaxLength)
}
