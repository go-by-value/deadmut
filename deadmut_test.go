package deadmut_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/go-by-value/deadmut"
)

func TestAnalyzer(t *testing.T) {
	t.Parallel()

	analysistest.Run(t, analysistest.TestData(), deadmut.NewAnalyzer(), "a", "b")
}
