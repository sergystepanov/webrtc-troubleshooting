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
		// !to move messages into signal
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

			message := ICE{
				T:       MessageICE,
				Payload: c.ToJSON(),
			}
			outbound, err := json.Marshal(message)
			if err != nil {
				_log("err: %v", err)
				return
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

		// !to make sure that the buffer is enough
		buf := make([]byte, 1500)
		for {
			n, err := ws.Read(buf)
			if err != nil {
				_log("err: %v", err)
				return
			}

			var m Message
			if err = json.Unmarshal(buf[:n], &m); err != nil {
				_log("err: %v, unknown message: %v", err, buf)
				continue
			}

			switch m.T {
			case MessageOffer:
				var offer webrtc.SessionDescription
				if json.Unmarshal(m.Payload, &offer) == nil && offer.SDP != "" {
					if err = peer.SetRemoteDescription(offer); err != nil {
						_log("err: %v", err)
						return
					}
				}
				answer, err := peer.CreateAnswer(nil)
				if err != nil {
					_log("err: %v", err)
					return
				}
				if err = peer.SetLocalDescription(answer); err != nil {
					_log("err: %v", err)
					return
				}
				message := Answer{
					T:       MessageAnswer,
					Payload: answer,
				}
				outbound, err := json.Marshal(message)
				if err != nil {
					_log("err: %v", err)
					return
				}
				if _, err = ws.Write(outbound); err != nil {
					_log("err: %v", err)
					return
				}
			case MessageICE:
				var candidate webrtc.ICECandidateInit
				if json.Unmarshal(m.Payload, &candidate) == nil && candidate.Candidate != "" {
					if err = peer.AddICECandidate(candidate); err != nil {
						_log("err: %v", err)
						return
					}
				}
			default:
				_log("err: unknown message [%v]", m.T)
				return
			}
		}
	}
}
