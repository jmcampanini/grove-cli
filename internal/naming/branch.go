package naming

import "github.com/jmcampanini/grove-cli/internal/config"

// BranchNameGenerator creates branch names from user input.
type BranchNameGenerator struct {
	prefix      string
	slugifyOpts SlugifyOptions
}

// NewBranchNameGenerator creates a generator from config.
func NewBranchNameGenerator(branchCfg config.BranchConfig, slugCfg config.SlugifyConfig) *BranchNameGenerator {
	return &BranchNameGenerator{
		prefix: branchCfg.NewPrefix,
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

// Generate creates a branch name from a phrase.
func (g *BranchNameGenerator) Generate(phrase string) string {
	slug := Slugify(phrase, g.slugifyOpts)
	if slug == "" {
		return ""
	}
	return g.prefix + slug
}
