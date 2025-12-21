package naming

import (
	"strings"

	"github.com/jmcampanini/grove-cli/internal/config"
)

// WorktreeNameGenerator creates worktree directory names.
type WorktreeNameGenerator struct {
	prefix            string
	slugifyOpts       SlugifyOptions
	stripBranchPrefix []string
}

// NewWorktreeNameGenerator creates a generator from config.
func NewWorktreeNameGenerator(worktreeCfg config.WorktreeConfig, slugCfg config.SlugifyConfig) *WorktreeNameGenerator {
	return &WorktreeNameGenerator{
		prefix:            worktreeCfg.NewPrefix,
		stripBranchPrefix: worktreeCfg.StripBranchPrefix,
		slugifyOpts: SlugifyOptions{
			CollapseDashes:     slugCfg.CollapseDashes,
			HashLength:         slugCfg.HashLength,
			Lowercase:          slugCfg.Lowercase,
			MaxLength:          slugCfg.MaxLength,
			ReplaceNonAlphaNum: slugCfg.ReplaceNonAlphanum,
			TrimDashes:         slugCfg.TrimDashes,
		},
	}
}

// Generate creates a worktree name from a branch name.
func (g *WorktreeNameGenerator) Generate(branchName string) string {
	if branchName == "" {
		return ""
	}

	name := branchName
	for _, prefix := range g.stripBranchPrefix {
		if strings.HasPrefix(name, prefix) {
			name = strings.TrimPrefix(name, prefix)
			break
		}
	}

	slug := Slugify(name, g.slugifyOpts)
	if slug == "" {
		return ""
	}

	return g.prefix + slug
}
