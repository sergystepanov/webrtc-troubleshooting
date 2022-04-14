package signal

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/pion/webrtc/v3"
	"golang.org/x/net/websocket"
)

func Signalling() websocket.Handler {
	return func(ws *websocket.Conn) {
		done := make(chan bool)
		messages := make(chan string, 100)

		_log := func(format string, v ...any) {
			m := fmt.Sprintf(format, v...)
			log.Printf(m)
			messages <- m
		}

		peer, err := webrtc.NewPeerConnection(webrtc.Configuration{})
		if err != nil {
			panic(err)
		}

		peer.OnICECandidate(func(c *webrtc.ICECandidate) {
			if c == nil {
				return
			}

			outbound, marshalErr := json.Marshal(c.ToJSON())
			if marshalErr != nil {
				panic(marshalErr)
			}

			if _, err = ws.Write(outbound); err != nil {
				panic(err)
			}
		})

		peer.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
			_log("ICE Connection State has changed: %s\n", connectionState.String())
		})

		peer.OnDataChannel(func(d *webrtc.DataChannel) {
			send := func(message string) {
				if d.ReadyState() == webrtc.DataChannelStateOpen {
					if err = d.SendText(message); err != nil {
						panic(err)
					}
				}
			}
			d.OnOpen(func() {
				send(time.Now().String())

				ticker := time.NewTicker(2000 * time.Millisecond)
				defer ticker.Stop()

				for {
					select {
					case <-done:
						_log("STOP TICKER!")
						return
					case t := <-ticker.C:
						_log("state: %v - t", d.ReadyState())
						send(t.String())
					case m := <-messages:
						send(m)
					}
				}
			})
		})

		defer func() {
			// !to wait for message drain
			done <- true
			err := peer.Close()
			if err != nil {
				log.Printf("err: %v", err)
				return
			}
		}()

		buf := make([]byte, 1500)
		for {
			n, err := ws.Read(buf)
			if err != nil {
				log.Printf("err: %v", err)
				return
			}

			var (
				candidate webrtc.ICECandidateInit
				offer     webrtc.SessionDescription
			)

			switch {
			case json.Unmarshal(buf[:n], &offer) == nil && offer.SDP != "":
				if err = peer.SetRemoteDescription(offer); err != nil {
					log.Printf("err: %v", err)
					return
				}

				answer, answerErr := peer.CreateAnswer(nil)
				if answerErr != nil {
					log.Printf("err: %v", err)
					return
				}

				if err = peer.SetLocalDescription(answer); err != nil {
					log.Printf("err: %v", err)
					return
				}

				outbound, marshalErr := json.Marshal(answer)
				if marshalErr != nil {
					log.Printf("err: %v", err)
					return
				}

				if _, err = ws.Write(outbound); err != nil {
					log.Printf("err: %v", err)
					return
				}
			case json.Unmarshal(buf[:n], &candidate) == nil && candidate.Candidate != "":
				if err = peer.AddICECandidate(candidate); err != nil {
					log.Printf("err: %v", err)
					return
				}
			default:
				log.Printf("err: %v", "Unknown message")
				return
			}
		}
	}
}
