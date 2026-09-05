package main

import (
	"log"
	"os"

	"yomirelay/internal/app"
)

func main() {
	logger := log.New(os.Stderr, "[backend] ", log.LstdFlags)
	if err := app.Main(os.Getenv, logger); err != nil {
		logger.Fatal(err)
	}
}
