package main

import (
	"flag"
	"net"

	"github.com/getlantern/waddell"
)

var (
	addr = flag.String("addr", ":62443", "host:port on which to listen for client connections")
)

func main() {
	server := &waddell.Server{}
	log.Printf("Starting waddell at %s", *addr)
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("Unable to listen at %s: %s", *addr, err)
	}
	err = server.Serve(listener)
	if err != nil {
		log.Fatalf("Unable to start waddell at %s: %s", *addr, err)
	}
}
