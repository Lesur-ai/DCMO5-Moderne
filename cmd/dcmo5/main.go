package main

import (
	"fmt"
	"os"

	"github.com/Lesur-ai/dcmo5/internal/app"
)

func main() {
	if err := app.Run(app.New()); err != nil {
		fmt.Fprintln(os.Stderr, "dcmo5:", err)
		os.Exit(1)
	}
}
