package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/sergystepanov/webrtc-troubleshooting/v2/internal/webui"
)

func main() {
	// read cmd flags
	webAddress := flag.String("addr", ":3000", "a web server address")
	flag.Parse()

	index, err := webui.Index()
	if err != nil {
		log.Fatalf("web content fail, %v", err)
	}
	http.Handle("/", index)

	log.Printf("Listening on %s...", *webAddress)
	if err = http.ListenAndServe(*webAddress, nil); err != nil {
		log.Fatal(err)
	}
}
