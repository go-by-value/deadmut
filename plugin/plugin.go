// Package plugin registers deadmut as a golangci-lint module plugin.
//
// Reference it from .custom-gcl.yml:
//
//	plugins:
//	  - module: github.com/go-by-value/deadmut
//	    import: github.com/go-by-value/deadmut/plugin
//	    version: v0.1.0
package plugin

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/go-by-value/deadmut"
)

func init() {
	register.Plugin("deadmut", New)
}

type plugin struct{}

var _ register.LinterPlugin = (*plugin)(nil)

// New returns the deadmut plugin. deadmut has no settings.
func New(_ any) (register.LinterPlugin, error) {
	return plugin{}, nil
}

func (plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{deadmut.NewAnalyzer()}, nil
}

func (plugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}
