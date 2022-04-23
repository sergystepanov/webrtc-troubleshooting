package signal

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/sergystepanov/webrtc-troubleshooting/v2/pkg/api"
	"golang.org/x/net/websocket"
)

func Signalling() websocket.Handler {
	send, receive := websocket.JSON.Send, websocket.JSON.Receive
	status := func() string { return fmt.Sprintf("%08b", rand.Intn(256)) }

	return func(conn *websocket.Conn) {
		messages, done := make(chan api.Log, 100), make(chan struct{})

		go func() {
			for {
				select {
				case <-done:
					log.Printf("SIGNAL STOP!")
					return
				case m := <-messages:
					if err := send(conn, api.NewLog(m)); err == nil {
						log.Printf("err: %v", err)
						return
					}
				}
			}
		}()

		_log := func(tag string, format string, v ...any) {
			m := fmt.Sprintf(format, v...)
			log.Printf(m)
			messages <- api.Log{Tag: tag, Text: m}
		}

		peer, err := webrtc.NewPeerConnection(webrtc.Configuration{})
		if err != nil {
			panic(err)
		}

		peer.OnICECandidate(func(c *webrtc.ICECandidate) {
			if c != nil {
				if err := send(conn, api.NewIce(*c)); err != nil {
					panic(err)
				}
			}
		})

		peer.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) { _log("ice", "→ %s", state) })
		peer.OnConnectionStateChange(func(state webrtc.PeerConnectionState) { _log("rtc", "→ %s", state) })
		peer.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) { _log("ice", "→ %s", state) })
		peer.OnSignalingStateChange(func(state webrtc.SignalingState) { _log("sig", "→ %s", state) })

		peer.OnDataChannel(func(d *webrtc.DataChannel) {
			send := func(message string) {
				if d.ReadyState() == webrtc.DataChannelStateOpen {
					if err = d.SendText(message); err != nil {
						panic(err)
					}
				}
			}
			d.OnOpen(func() {
				send(status())
				ticker := time.NewTicker(10 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-done:
						_log("sys", "STOP TICKER!")
						return
					case _ = <-ticker.C:
						send(status())
					}
				}
			})
		})

		defer func() {
			// !to wait for message drain
			done <- struct{}{}
			err := peer.Close()
			if err != nil {
				log.Printf("err: %v", err)
				return
			}
		}()

		for {
			var m api.Message
			err := receive(conn, &m)
			if errors.Is(err, io.EOF) {
				log.Printf("Signal has been closed!")
				return
			} else if err != nil {
				_log("sys", "err: %v", err)
				continue
			}

			switch m.T {
			case api.WebrtcOffer:
				if offer, err := api.NewSessionDescription(m.Payload); err == nil && offer.SDP != "" {
					if err = peer.SetRemoteDescription(offer.SessionDescription); err != nil {
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
				if err = send(conn, api.NewAnswer(answer)); err != nil {
					_log("rtc", "err: %v", err)
					return
				}
			case api.WebrtcIce:
				if candidate, err := api.NewIceCandidateInit(m.Payload); err == nil && candidate.Candidate != "" {
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
