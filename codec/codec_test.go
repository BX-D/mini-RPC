package codec

import (
	"mini-rpc/message"
	"testing"
)

func TestJSONCodec(t *testing.T) {
	// Create a JSONCodec instance
	jsonCodec := &JSONCodec{}

	// Prepare a RPCMessage for testing
	originalMsg := &message.RPCMessage{
		ServiceMethod: "ArithService.Add",
		Payload:       []byte(`{"a":1,"b":2}`),
		Error:         "",
	}

	// Encode the message
	data, err := jsonCodec.Encode(originalMsg)
	if err != nil {
		t.Fatalf("JSONCodec Encode failed: %v", err)
	}

	// Decode the message back
	var decodedMsg message.RPCMessage
	err = jsonCodec.Decode(data, &decodedMsg)
	if err != nil {
		t.Fatalf("JSONCodec Decode failed: %v", err)
	}

	// Verify that the original and decoded messages are the same
	if originalMsg.ServiceMethod != decodedMsg.ServiceMethod {
		t.Errorf("ServiceMethod mismatch: got %s, want %s", decodedMsg.ServiceMethod, originalMsg.ServiceMethod)
	}
	if string(originalMsg.Payload) != string(decodedMsg.Payload) {
		t.Errorf("Payload mismatch: got %s, want %s", string(decodedMsg.Payload), string(originalMsg.Payload))
	}
	if originalMsg.Error != decodedMsg.Error {
		t.Errorf("Error mismatch: got %s, want %s", decodedMsg.Error, originalMsg.Error)
	}

	t.Logf("Pass all the test for JSONCodec!")
}

func TestBinaryCodec(t *testing.T) {
	binaryCodec := &BinaryCodec{}

	originalMsg := &message.RPCMessage{
		ServiceMethod: "ArithService.Add",
		Payload:       []byte(`{"a":1,"b":2}`),
		Error:         "",
	}

	data, err := binaryCodec.Encode(originalMsg)
	if err != nil {
		t.Fatalf("BinaryCodec Encode failed: %v", err)
	}

	var decodedMsg message.RPCMessage
	err = binaryCodec.Decode(data, &decodedMsg)
	if err != nil {
		t.Fatalf("BinaryCodec Decode failed: %v", err)
	}

	if originalMsg.ServiceMethod != decodedMsg.ServiceMethod {
		t.Errorf("ServiceMethod mismatch: got %s, want %s", decodedMsg.ServiceMethod, originalMsg.ServiceMethod)
	}
	if string(originalMsg.Payload) != string(decodedMsg.Payload) {
		t.Errorf("Payload mismatch: got %s, want %s", string(decodedMsg.Payload), string(originalMsg.Payload))
	}
	if originalMsg.Error != decodedMsg.Error {
		t.Errorf("Error mismatch: got %s, want %s", decodedMsg.Error, originalMsg.Error)
	}

	t.Logf("Pass all the test for BinaryCodec!")
}