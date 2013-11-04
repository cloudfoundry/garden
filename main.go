package main

import (
	"flag"
	"log"

	"github.com/vito/garden/backend/fakebackend"
	"github.com/vito/garden/server"
)

var socketFilePath = flag.String(
	"socket",
	"/tmp/warden.sock",
	"where to put the wardern server .sock file",
)

func main() {
	flag.Parse()

	wardenServer := server.New(*socketFilePath, fakebackend.New())

	err := wardenServer.Start()
	if err != nil {
		log.Fatalln("failed to start:", err)
	}

	select {}
}
