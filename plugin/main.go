package main

import (
	"github.com/sqlc-dev/sqlc-go/codegen"

	golang "github.com/sqlc-dev/sqlc-gen-go/internal"
)

func main() {
	codegen.Run(golang.Generate)
}
