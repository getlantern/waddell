package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

func main() {
	var wg sync.WaitGroup
	wg.Add(1)
	http.HandleFunc("/", handle)
	go func() {
		err := http.ListenAndServe(os.Args[1], nil)
		if err != nil {
			log.Fatalf("Unable to start server: %s", err)
		}
		wg.Done()
	}()
	wg.Wait()
}

func handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	for i := 0; i < 500; i++ {
		fmt.Fprintf(w, "line%d\n", i)
		w.(http.Flusher).Flush()
		time.Sleep(1 * time.Second)
	}
}
