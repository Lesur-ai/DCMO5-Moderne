package main

import (
	"fmt"
	"os"

	"github.com/Lesur-ai/dcmo5/internal/app"
	"github.com/Lesur-ai/dcmo5/internal/core"
)

func main() {
	machine, err := core.NewMachine(core.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "dcmo5: init machine:", err)
		os.Exit(1)
	}
	machine.Reset()

	if err := app.Run(app.New(machine)); err != nil {
		fmt.Fprintln(os.Stderr, "dcmo5:", err)
		os.Exit(1)
	}
}
