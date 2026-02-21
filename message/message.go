// Package message defines the RPC message structure exchanged between client and server.
//
// RPCMessage is the "envelope" for every RPC call. It gets serialized by the codec layer
// and wrapped in a protocol frame for transmission over TCP.
package message

// RPCMessage carries the data for a single RPC request or response.
//
//   - On request:  ServiceMethod is set, Payload contains the serialized args, Error is empty.
//   - On response: Payload contains the serialized reply, Error is non-empty if the call failed.
type RPCMessage struct {
	ServiceMethod string // Format: "ServiceName.MethodName", e.g., "Arith.Add"
	Error         string // Non-empty if the server-side handler returned an error
	Payload       []byte // Serialized args (request) or reply (response) as JSON bytes
}
