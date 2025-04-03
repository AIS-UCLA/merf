package main

import (
	"fmt"
	"github.com/ais-ucla/merf/client"
	"github.com/ais-ucla/merf/server"
	"os"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: merf <server|client> [options]\n")
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	switch os.Args[1] {
	case "server":
		server.Main()
	case "client":
		client.Main()
	default:
		usage()
	}
}
