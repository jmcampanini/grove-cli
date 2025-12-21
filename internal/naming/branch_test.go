package naming

import (
	"testing"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestBranchNameGenerator_Generate(t *testing.T) {
	tests := []struct {
		name       string
		branchCfg  config.BranchConfig
		slugifyCfg config.SlugifyConfig
		phrase     string
		want       string
	}{
		{
			name:       "simple phrase with feature prefix",
			branchCfg:  config.BranchConfig{NewPrefix: "feature/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "add user auth",
			want:       "feature/add-user-auth",
		},
		{
			name:       "phrase with fix prefix",
			branchCfg:  config.BranchConfig{NewPrefix: "fix/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "login bug",
			want:       "fix/login-bug",
		},
		{
			name:       "empty prefix",
			branchCfg:  config.BranchConfig{NewPrefix: ""},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "my feature",
			want:       "my-feature",
		},
		{
			name:       "phrase with special characters",
			branchCfg:  config.BranchConfig{NewPrefix: "feature/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "add @auth & login!",
			want:       "feature/add-auth-login",
		},
		{
			name:       "phrase with numbers",
			branchCfg:  config.BranchConfig{NewPrefix: "feature/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "fix issue 123",
			want:       "feature/fix-issue-123",
		},
		{
			name:       "phrase with uppercase",
			branchCfg:  config.BranchConfig{NewPrefix: "feature/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "Add User AUTH",
			want:       "feature/add-user-auth",
		},
		{
			name:       "empty phrase returns empty",
			branchCfg:  config.BranchConfig{NewPrefix: "feature/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "",
			want:       "",
		},
		{
			name:       "phrase with only special chars returns empty",
			branchCfg:  config.BranchConfig{NewPrefix: "feature/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "@#$%^&*()",
			want:       "",
		},
		{
			name:      "long phrase gets truncated with hash",
			branchCfg: config.BranchConfig{NewPrefix: "feature/"},
			slugifyCfg: config.SlugifyConfig{
				CollapseDashes:     true,
				HashLength:         4,
				Lowercase:          true,
				MaxLength:          20,
				ReplaceNonAlphanum: true,
				TrimDashes:         true,
			},
			phrase: "this is a very long feature description that should be truncated",
			want:   "feature/this-is-a-very-xolx", // prefix + truncated slug with hash
		},
		{
			name:       "phrase with unicode",
			branchCfg:  config.BranchConfig{NewPrefix: "feature/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "add emoji support",
			want:       "feature/add-emoji-support",
		},
		{
			name:       "phrase with dashes already",
			branchCfg:  config.BranchConfig{NewPrefix: "feature/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "my-feature-name",
			want:       "feature/my-feature-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewBranchNameGenerator(tt.branchCfg, tt.slugifyCfg)
			got := gen.Generate(tt.phrase)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewBranchNameGenerator(t *testing.T) {
	branchCfg := config.BranchConfig{NewPrefix: "test/"}
	slugCfg := config.SlugifyConfig{
		CollapseDashes:     true,
		HashLength:         6,
		Lowercase:          false,
		MaxLength:          100,
		ReplaceNonAlphanum: true,
		TrimDashes:         false,
	}

	gen := NewBranchNameGenerator(branchCfg, slugCfg)

	assert.Equal(t, "test/", gen.prefix)
	assert.Equal(t, 6, gen.slugifyOpts.HashLength)
	assert.False(t, gen.slugifyOpts.Lowercase)
	assert.Equal(t, 100, gen.slugifyOpts.MaxLength)
}

// defaultSlugifyConfig returns the default slugify config for testing
func defaultSlugifyConfig() config.SlugifyConfig {
	return config.SlugifyConfig{
		CollapseDashes:     true,
		HashLength:         4,
		Lowercase:          true,
		MaxLength:          50,
		ReplaceNonAlphanum: true,
		TrimDashes:         true,
	}
}
