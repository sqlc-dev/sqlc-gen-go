all: sqlc-gen-go sqlc-gen-go.wasm

sqlc-gen-go:
	cd plugin && go build -o ~/bin/sqlc-gen-go ./main.go

sqlc-gen-go.wasm:
	cd plugin && GOOS=wasip1 GOARCH=wasm go build -o sqlc-gen-go.wasm main.go
	openssl sha256 plugin/sqlc-gen-go.wasm
