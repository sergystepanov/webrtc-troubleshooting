package signal

import (
	"encoding/json"
	"github.com/pion/webrtc/v3"
)

type Message struct {
	T       MessageType     `json:"t"`
	Payload json.RawMessage `json:"p,omitempty"`
}

type MessageType string

const (
	MessageICE    MessageType = "ICE"
	MessageOffer  MessageType = "OFFER"
	MessageAnswer MessageType = "ANSWER"
)

type (
	Answer struct {
		T       MessageType               `json:"t"`
		Payload webrtc.SessionDescription `json:"p"`
	}
	ICE struct {
		T       MessageType             `json:"t"`
		Payload webrtc.ICECandidateInit `json:"p"`
	}
)
