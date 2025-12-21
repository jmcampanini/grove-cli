package shell

import (
	_ "embed"
)

//go:embed scripts/grc.fish
var fishScript string

//go:embed scripts/grc.bash
var bashScript string

//go:embed scripts/grc.zsh
var zshScript string

// FunctionGenerator generates shell functions.
type FunctionGenerator struct{}

// NewFunctionGenerator creates a new FunctionGenerator.
func NewFunctionGenerator() *FunctionGenerator {
	return &FunctionGenerator{}
}

// GenerateFish returns the fish shell function.
func (g *FunctionGenerator) GenerateFish() string {
	return fishScript
}

// GenerateZsh returns the zsh shell function.
func (g *FunctionGenerator) GenerateZsh() string {
	return zshScript
}

// GenerateBash returns the bash shell function.
func (g *FunctionGenerator) GenerateBash() string {
	return bashScript
}
