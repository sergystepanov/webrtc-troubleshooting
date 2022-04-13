package signal

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pion/webrtc/v3"
	"golang.org/x/net/websocket"
)

func Signalling() websocket.Handler { return WebsocketServer }

// WebsocketServer is called for every new inbound WebSocket
func WebsocketServer(ws *websocket.Conn) {
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		panic(err)
	}

	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
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

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		d.OnOpen(func() {
			for range time.Tick(time.Second * 3) {
				if err = d.SendText(time.Now().String()); err != nil {
					panic(err)
				}
			}
		})
	})

	buf := make([]byte, 1500)
	for {
		// Read each inbound WebSocket Message
		n, err := ws.Read(buf)
		if err != nil {
			panic(err)
		}

		// Unmarshal each inbound WebSocket message
		var (
			candidate webrtc.ICECandidateInit
			offer     webrtc.SessionDescription
		)

		switch {
		// Attempt to unmarshal as a SessionDescription. If the SDP field is empty
		// assume it is not one.
		case json.Unmarshal(buf[:n], &offer) == nil && offer.SDP != "":
			if err = peerConnection.SetRemoteDescription(offer); err != nil {
				panic(err)
			}

			answer, answerErr := peerConnection.CreateAnswer(nil)
			if answerErr != nil {
				panic(answerErr)
			}

			if err = peerConnection.SetLocalDescription(answer); err != nil {
				panic(err)
			}

			outbound, marshalErr := json.Marshal(answer)
			if marshalErr != nil {
				panic(marshalErr)
			}

			if _, err = ws.Write(outbound); err != nil {
				panic(err)
			}
		// Attempt to unmarshal as a ICECandidateInit. If the candidate field is empty
		// assume it is not one.
		case json.Unmarshal(buf[:n], &candidate) == nil && candidate.Candidate != "":
			if err = peerConnection.AddICECandidate(candidate); err != nil {
				panic(err)
			}
		default:
			panic("Unknown message")
		}
	}
}
