package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
)

func main() {
	client := http.Client{
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return url.Parse("http://localhost:10080")
			},
		},
	}
	resp, err := client.Get(fmt.Sprintf("http://%s/", os.Args[1]))
	if err != nil {
		log.Fatalf("Unable to open request: %s", err)
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
}
