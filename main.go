// deadmut detects mutations to range loop value copies that have no effect.
package main

import (
	"fmt"
	"os"

	"github.com/mickamy/deadmut/internal/analyzer"
	"golang.org/x/tools/go/analysis/singlechecker"
)

var version = "dev"

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "-V" || arg == "--version" {
			fmt.Printf("deadmut version %s\n", version)
			os.Exit(0)
		}
	}
	singlechecker.Main(analyzer.Analyzer)
}
