package protocol

import "encoding/json"

type Request struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data,omitempty"`
}

type Response struct {
	OK     bool        `json:"ok"`
	Action string      `json:"action"`
	Error  string      `json:"error,omitempty"`
	Data   any         `json:"data,omitempty"`
}