package main

import (
	"os"

	"github.com/andrius/gmail-cli/internal/app"
)

func main() {
	os.Exit(app.Main(os.Args[1:]))
}
