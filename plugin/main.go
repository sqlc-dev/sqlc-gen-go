package main

import (
	"github.com/sqlc-dev/plugin-sdk-go/codegen"
	"io"
	"os"

	golang "github.com/sqlc-dev/sqlc-gen-go/internal"
)

func main() {
	dumpFile := os.Getenv("DUMP_REQUEST_FILE")
	restoreFile := os.Getenv("RESTORE_REQUEST_FILE")

	if dumpFile != "" {
		reqBytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			panic(err)
		}

		err = os.WriteFile(dumpFile, reqBytes, 0644)
		if err != nil {
			panic(err)
		}

		fd, err := os.Open(dumpFile)
		if err != nil {
			panic(err)
		}
		os.Stdin = fd
	}
	if restoreFile != "" {
		fd, err := os.Open(restoreFile)
		if err != nil {
			panic(err)
		}
		os.Stdin = fd
	}

	codegen.Run(golang.Generate)
}
