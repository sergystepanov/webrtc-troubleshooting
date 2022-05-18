package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/sergystepanov/webrtc-troubleshooting/v2/internal/signal"
	"github.com/sergystepanov/webrtc-troubleshooting/v2/internal/webui"
)

func main() {
	// read cmd flags
	live := flag.Bool("live", true, "use live webui")
	addr := flag.String("addr", ":3000", "a web server address")
	flag.Parse()

	index, err := webui.Index(*live)
	if err != nil {
		log.Fatalf("web content fail, %v", err)
	}

	http.Handle("/", index)
	http.Handle("/websocket", signal.Handler())

	log.Printf("Listening on %s...", *addr)
	if err = http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal(err)
	}
}
