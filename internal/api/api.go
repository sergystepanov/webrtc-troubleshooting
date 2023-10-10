package api

import (
	"encoding/json"
	"time"

	"github.com/pion/webrtc/v4"
)

const (
	WebrtcIce          MessageType = "ICE"
	WebrtcOffer        MessageType = "OFFER"
	WebrtcAnswer       MessageType = "ANSWER"
	WebrtcWaitingOffer MessageType = "WAITING_OFFER"
	WebrtcClose        MessageType = "CLOSE"
	MessageLog         MessageType = "LOG"
)

type (
	ICE struct {
		typed
		Payload webrtc.ICECandidateInit `json:"p"`
	}
	Close struct {
		typed
	}
	Log struct {
		Tag  string    `json:"tag"`
		Time time.Time `json:"time"`
		Text string    `json:"text"`
	}
	LogMessage struct {
		typed
		Payload Log `json:"p"`
	}
	Message struct {
		typed
		Payload json.RawMessage `json:"p,omitempty"`
	}
	MessageType        string
	SessionDescription struct {
		webrtc.SessionDescription
	}
	// SDP answer/offer
	SDP struct {
		typed
		Payload webrtc.SessionDescription `json:"p"`
	}
	typed struct {
		T MessageType `json:"t"`
	}
)

func NewSessionDescription(data []byte) (*SessionDescription, error) {
	var sess SessionDescription
	err := json.Unmarshal(data, &sess)
	return &sess, err
}

func NewIceCandidateInit(data []byte) (webrtc.ICECandidateInit, error) {
	var candidate webrtc.ICECandidateInit
	err := json.Unmarshal(data, &candidate)
	return candidate, err
}

func NewSDP(s webrtc.SessionDescription, t MessageType) SDP {
	return SDP{typed: typed{t}, Payload: s}
}

func NewIce(candidate webrtc.ICECandidate) ICE {
	return ICE{typed: typed{WebrtcIce}, Payload: candidate.ToJSON()}
}
func NewLog(l Log) LogMessage {
	return LogMessage{
		typed{MessageLog},
		Log{
			Tag:  l.Tag,
			Time: time.Now(),
			Text: l.Text,
		},
	}
}

func NewClose() Close { return Close{typed{WebrtcClose}} }
