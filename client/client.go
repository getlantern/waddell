package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	resp, err := http.Get(fmt.Sprintf("http://%s/", os.Args[1]))
	if err != nil {
		log.Fatalf("Unable to open request: %s", err)
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
}
