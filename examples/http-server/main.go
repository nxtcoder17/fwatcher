package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	var addr string
	flag.StringVar(&addr, "addr", ":3000", "--addr <host:port>")
	flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})

	log.Printf("http-server listening on addr %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
