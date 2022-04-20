package signal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/pion/webrtc/v3"
	"golang.org/x/net/websocket"
)

const socketReadBuffer = 1500

func Signalling() websocket.Handler {
	return func(conn *websocket.Conn) {
		done := make(chan bool)
		// !to move messages into signal
		messages := make(chan Log, 100)

		go func() {
			defer log.Printf("STOP SIGNALE ROUTINE")
			for {
				select {
				case <-done:
					log.Printf("SIGNAL STOP!")
					return
				case m := <-messages:
					if dat, err := NewLog(m); err == nil {
						if _, err := conn.Write(dat); err != nil {
							log.Printf("err: %v", err)
							return
						}
					}
				}
			}
		}()

		_log := func(tag string, format string, v ...any) {
			m := fmt.Sprintf(format, v...)
			log.Printf(m)
			messages <- Log{Tag: tag, Text: m}
		}

		peer, err := webrtc.NewPeerConnection(webrtc.Configuration{})
		if err != nil {
			panic(err)
		}

		peer.OnICECandidate(func(c *webrtc.ICECandidate) {
			if c == nil {
				return
			}
			outbound, err := NewIce(*c)
			if err != nil {
				_log("ice", "err: %v", err)
				return
			}
			if _, err = conn.Write(outbound); err != nil {
				panic(err)
			}
		})

		peer.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
			_log("ice", "→ %s", connectionState)
		})

		peer.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
			_log("rtc", "→ %s", state)
		})

		peer.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) {
			_log("ice", "→ %s", state)
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
						_log("sys", "STOP TICKER!")
						return
					case t := <-ticker.C:
						send(t.String())
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
		buf := make([]byte, socketReadBuffer)
		for {
			n, err := conn.Read(buf)
			if errors.Is(err, io.EOF) {
				log.Printf("Signal has been closed!")
				return
			} else if err != nil {
				_log("sys", "err: %v", err)
				return
			}

			var m Message
			if err = json.Unmarshal(buf[:n], &m); err != nil {
				_log("sys", "err: %v, unknown message: %v", err, buf)
				continue
			}

			switch m.T {
			case MessageOffer:
				var offer webrtc.SessionDescription
				if json.Unmarshal(m.Payload, &offer) == nil && offer.SDP != "" {
					if err = peer.SetRemoteDescription(offer); err != nil {
						_log("rtc", "err: %v", err)
						return
					}
				}
				answer, err := peer.CreateAnswer(nil)
				if err != nil {
					_log("rtc", "err: %v", err)
					return
				}
				if err = peer.SetLocalDescription(answer); err != nil {
					_log("rtc", "err: %v", err)
					return
				}

				outbound, err := NewAnswer(answer)
				if err != nil {
					_log("rtc", "err: %v", err)
					return
				}
				if _, err = conn.Write(outbound); err != nil {
					_log("rtc", "err: %v", err)
					return
				}
			case MessageICE:
				var candidate webrtc.ICECandidateInit
				if json.Unmarshal(m.Payload, &candidate) == nil && candidate.Candidate != "" {
					if err = peer.AddICECandidate(candidate); err != nil {
						_log("ice", "err: %v", err)
						return
					}
				}
			default:
				_log("sys", "err: unknown message [%v]", m.T)
				return
			}
		}
	}
}
