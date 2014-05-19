package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	//"net/url"
	"os"
)

func main() {
	client := http.Client{}s
	http.NewRequest("GET", fmt.Sprintf("http://%s/", os.Args[1]), nil)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Unable to open request: %s", err)
	}
	log.Println(resp.TransferEncoding)
	defer resp.Body.Close()
	go func() {
		io.Copy(os.Stdout, resp.Body)
		wg.Done()
		}()
	for {
		re
	}
}
