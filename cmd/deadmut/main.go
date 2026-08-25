// Command deadmut reports mutations of range loop value copies that have no
// effect.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/go-by-value/deadmut"
)

func main() {
	singlechecker.Main(deadmut.NewAnalyzer())
}
