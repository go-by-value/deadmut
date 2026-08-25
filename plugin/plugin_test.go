package plugin_test

import (
	"testing"

	"github.com/golangci/plugin-module-register/register"

	_ "github.com/go-by-value/deadmut/plugin"
)

func TestRegistered(t *testing.T) {
	t.Parallel()

	newPlugin, err := register.GetPlugin("deadmut")
	if err != nil {
		t.Fatalf("GetPlugin: %v", err)
	}

	p, err := newPlugin(nil)
	if err != nil {
		t.Fatalf("newPlugin: %v", err)
	}

	analyzers, err := p.BuildAnalyzers()
	if err != nil {
		t.Fatalf("BuildAnalyzers: %v", err)
	}
	if len(analyzers) != 1 || analyzers[0].Name != "deadmut" {
		t.Fatalf("BuildAnalyzers: got %v, want one analyzer named deadmut", analyzers)
	}

	if got := p.GetLoadMode(); got != register.LoadModeTypesInfo {
		t.Fatalf("GetLoadMode: got %q, want %q", got, register.LoadModeTypesInfo)
	}
}
