// Package codec provides the serialization layer for mini-RPC.
//
// It defines a pluggable Codec interface with two implementations:
//   - JSONCodec:   human-readable, easy to debug, slower (~589 ns/op)
//   - BinaryCodec: compact binary format, faster (~65 ns/op, ~9x speedup)
//
// The codec type is stored in the protocol frame header so the receiver
// knows which codec to use for deserialization.
package codec

// CodecType identifies the serialization format, stored as 1 byte in the frame header.
type CodecType byte

const (
	CodecTypeJSON   CodecType = 0 // JSON serialization (encoding/json)
	CodecTypeBinary CodecType = 1 // Custom binary serialization
)

// Codec is the interface for serialization/deserialization.
// Implementing this interface allows adding new formats (e.g., Protobuf)
// without changing any other layer — this is the Strategy Pattern.
type Codec interface {
	Encode(v any) ([]byte, error)    // Serialize a struct to bytes
	Decode(data []byte, v any) error // Deserialize bytes back to a struct
	Type() CodecType                 // Return the codec type identifier
}

// GetCodec is a factory function that returns the appropriate codec by type.
func GetCodec(codecType CodecType) Codec {
	if codecType == CodecTypeJSON {
		return &JSONCodec{}
	}
	return &BinaryCodec{}
}
