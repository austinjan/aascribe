package main

import (
	"os"

	"github.com/austinjan/aascribe/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:], os.Stdout))
}
