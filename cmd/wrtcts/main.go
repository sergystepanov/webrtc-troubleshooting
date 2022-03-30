package main

import (
	"flag"
	"log"
	"net/http"

	html "github.com/sergystepanov/webrtc-troubleshooting/v2"
)

func main() {
	// read cmd flags
	webAddress := flag.String("addr", ":3000", "a web server address")
	flag.Parse()

	index, err := html.Index()
	if err != nil {
		log.Fatalf("web content fail, %v", err)
	}
	http.Handle("/", index)

	log.Printf("Listening on %s...", *webAddress)
	if err = http.ListenAndServe(*webAddress, nil); err != nil {
		log.Fatal(err)
	}
}
