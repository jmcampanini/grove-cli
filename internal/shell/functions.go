package shell

import (
	_ "embed"
)

//go:embed scripts/grc.fish
var grcFishScript string

//go:embed scripts/grc.bash
var grcBashScript string

//go:embed scripts/grc.zsh
var grcZshScript string

//go:embed scripts/grs.fish
var grsFishScript string

//go:embed scripts/grs.bash
var grsBashScript string

//go:embed scripts/grs.zsh
var grsZshScript string

// FunctionGenerator generates shell functions.
type FunctionGenerator struct{}

// NewFunctionGenerator creates a new FunctionGenerator.
func NewFunctionGenerator() *FunctionGenerator {
	return &FunctionGenerator{}
}

// GenerateFish returns all fish shell functions.
func (g *FunctionGenerator) GenerateFish() string {
	return grcFishScript + "\n" + grsFishScript
}

// GenerateZsh returns all zsh shell functions.
func (g *FunctionGenerator) GenerateZsh() string {
	return grcZshScript + "\n" + grsZshScript
}

// GenerateBash returns all bash shell functions.
func (g *FunctionGenerator) GenerateBash() string {
	return grcBashScript + "\n" + grsBashScript
}
