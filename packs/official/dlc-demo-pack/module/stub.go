//go:build !wasip1

// This stub keeps the module package buildable on the host toolchain so
// `go build ./...` does not fail with "build constraints exclude all Go files".
// The real WASM entry point lives in main.go (//go:build wasip1) and is compiled
// with: GOOS=wasip1 GOARCH=wasm go build -o ../module.wasm .
package main

func main() {}
