package main

import (
	"flag"
	"log"

	"github.com/getlantern/waddell"
)

var (
	addr = flag.String("addr", ":62443", "host:port on which to listen for client connections")
)

func main() {
	server := &waddell.Server{}
	log.Printf("Starting waddell at %s", *addr)
	err := server.ListenAndServe(*addr)
	if err != nil {
		log.Fatalf("Unable to start waddell at %s: %s", *addr, err)
	}
}
