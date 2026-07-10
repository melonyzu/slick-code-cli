// Command slickcode is the entrypoint for Slick Code CLI.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/melonyzu/slick-code-cli/internal/core"
	"github.com/melonyzu/slick-code-cli/internal/runtime"
	"github.com/melonyzu/slick-code-cli/internal/terminal"
)

func main() {
	os.Exit(run())
}

func run() int {
	app, err := core.Bootstrap()
	if err != nil {
		fmt.Fprintln(os.Stderr, "slickcode:", err)
		return 1
	}

	if err := runtime.New(app).Run(context.Background()); err != nil {
		fmt.Fprintln(app.Terminal.Err, terminal.Error.Render("Error: "+err.Error()))
		return 1
	}

	return 0
}
