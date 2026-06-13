package main

import (
	"os"

	clicmd "github.com/Thatooine/loyalty-points-app/cmd/cli/cmd"
)

func main() {
	if err := clicmd.Execute(); err != nil {
		os.Exit(1)
	}
}
