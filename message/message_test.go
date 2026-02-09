package message

import (
	"encoding/json"
	"fmt"
	"testing"
)

type AddArgs struct {
	A int `json:"a"`
	B int `json:"b"`
}

func TestRequestResponse(t *testing.T) {
	// Create a Request
	req := &RPCMessage{
		ServiceMethod: "ArithService.Add",
		Error:         "",
		Payload:       []byte(`{"a":1,"b":2}`), // 你知道这是 AddArgs
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	var req2 RPCMessage
	// Create a Response
	err = json.Unmarshal(data, &req2)

	if err != nil {
		t.Fatalf("Failed to unmarshal with error: %v", err)
	}

	fmt.Printf("Decoded Request: %+v\n", req2.Payload)
}
