package main

import (
	"fmt"
	// "io"
	"log"
	"net/http"
	// "net/url"
	"os"

	"github.com/getlantern/framed"
)

func main() {
	client := http.Client{
	// Transport: &http.Transport{
	// 	Proxy: func(req *http.Request) (*url.URL, error) {
	// 		return url.Parse("http://localhost:10080")
	// 	},
	// },
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/", os.Args[1]), nil)
	if err != nil {
		log.Fatalf("Unable to create request: %s", err)
	}
	req.Header.Set("X-Lantern-Streaming", "yes")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Unable to open request: %s", err)
	}
	log.Println(resp.TransferEncoding)
	defer resp.Body.Close()

	reader := &framed.Reader{resp.Body}
	for {
		frame := make([]byte, 5000)
		_, err := reader.Read(frame)
		if err != nil {
			return
		}
		log.Printf("Got frame: %s", frame)
	}
}
