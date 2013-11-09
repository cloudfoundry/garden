package main

import (
	"flag"
	"log"

	"github.com/vito/garden/backend/fake_backend"
	"github.com/vito/garden/server"
)

var socketFilePath = flag.String(
	"socket",
	"/tmp/warden.sock",
	"where to put the wardern server .sock file",
)

func main() {
	flag.Parse()

	wardenServer := server.New(*socketFilePath, fake_backend.New())

	err := wardenServer.Start()
	if err != nil {
		log.Fatalln("failed to start:", err)
	}

	select {}
}
