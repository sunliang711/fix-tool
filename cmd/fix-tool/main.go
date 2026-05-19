package main

import (
	"context"
	"os"

	"fix-tool/internal/app"
)

func main() {
	os.Exit(app.Run(context.Background(), os.Args[1:]))
}
