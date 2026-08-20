package myvocalwss

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type BaseResponse struct {
	StatusCode *int   `json:"status_code,omitempty"`
	StatusMsg  string `json:"status_msg,omitempty"`
}

type EventData struct {
	Audio    string          `json:"audio,omitempty"`
	Subtitle json.RawMessage `json:"subtitle,omitempty"`
}

type Event struct {
	Event     string          `json:"event"`
	SessionID string          `json:"session_id,omitempty"`
	ConnectID string          `json:"connect_id,omitempty"`
	TraceID   string          `json:"trace_id,omitempty"`
	BaseResp  BaseResponse    `json:"base_resp,omitempty"`
	ErrorCode interface{}     `json:"error_code,omitempty"`
	Error     string          `json:"error,omitempty"`
	Data      EventData       `json:"data,omitempty"`
	IsFinal   *bool           `json:"is_final,omitempty"`
	ExtraInfo json.RawMessage `json:"extra_info,omitempty"`
}

func (e Event) AudioBytes() ([]byte, error) {
	if e.Data.Audio == "" {
		return nil, nil
	}
	audio, err := hex.DecodeString(e.Data.Audio)
	if err != nil {
		return nil, fmt.Errorf("invalid hex audio: %w", err)
	}
	return audio, nil
}

type ProtocolError struct {
	ErrorCode     string
	StatusCode    *int
	StatusMessage string
	Message       string
}

func (e *ProtocolError) Error() string {
	code := e.ErrorCode
	if code == "" && e.StatusCode != nil {
		code = fmt.Sprint(*e.StatusCode)
	}
	if code == "" {
		code = "protocol_error"
	}
	message := e.Message
	if message == "" {
		message = e.StatusMessage
	}
	if message == "" {
		message = "task failed"
	}
	return code + ": " + message
}

func (e Event) protocolError() error {
	code := ""
	if e.ErrorCode != nil {
		code = fmt.Sprint(e.ErrorCode)
	}
	return &ProtocolError{
		ErrorCode:     code,
		StatusCode:    e.BaseResp.StatusCode,
		StatusMessage: e.BaseResp.StatusMsg,
		Message:       e.Error,
	}
}
