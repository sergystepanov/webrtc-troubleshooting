package signal

import (
	"encoding/json"
	"time"

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
	MessageLog    MessageType = "LOG"
)

type (
	TypedMessage struct {
		T MessageType `json:"t"`
	}
	Answer struct {
		TypedMessage
		Payload webrtc.SessionDescription `json:"p"`
	}
	ICE struct {
		TypedMessage
		Payload webrtc.ICECandidateInit `json:"p"`
	}
	Log struct {
		Time time.Time `json:"time"`
		Text string    `json:"text"`
	}
	LogMessage struct {
		TypedMessage
		Payload Log `json:"p"`
	}
)

func NewAnswer(sdp webrtc.SessionDescription) ([]byte, error) {
	return json.Marshal(Answer{TypedMessage: TypedMessage{T: MessageAnswer}, Payload: sdp})
}
func NewIce(candidate webrtc.ICECandidate) ([]byte, error) {
	return json.Marshal(ICE{TypedMessage: TypedMessage{T: MessageICE}, Payload: candidate.ToJSON()})
}
func NewLog(data string) ([]byte, error) {
	return json.Marshal(LogMessage{
		TypedMessage: TypedMessage{T: MessageLog},
		Payload: Log{
			Time: time.Now(),
			Text: data,
		},
	})
}
