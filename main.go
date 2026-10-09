package main

import (
	_ "embed"
	"seanime/internal/server"
)

//go:embed internal/icon/logo.png
var embeddedLogo []byte

func main() {
	server.StartServer(WebFS, embeddedLogo)
}
