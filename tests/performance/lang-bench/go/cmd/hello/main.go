// Command hello is a minimal net/http service used to measure idle RSS.
package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	addr := "127.0.0.1:18080"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}
	http.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("hello"))
	})
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
