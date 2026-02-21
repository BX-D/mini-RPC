package codec

import (
	"encoding/binary"
	"errors"
	"mini-rpc/message"
)

// BinaryCodec implements a custom binary serialization for RPCMessage.
//
// Binary format:
//
//	┌─────────────┬──────────────┬──────────────┬─────────┬────────────┬───────┐
//	│MethodLen(2) │ Method bytes │ PayloadLen(4)│ Payload │ ErrLen(2)  │ Error │
//	└─────────────┴──────────────┴──────────────┴─────────┴────────────┴───────┘
//
// Note: The payload itself (args/reply) is still JSON-encoded. The performance gain
// comes from encoding the outer RPCMessage fields in binary instead of JSON,
// avoiding the overhead of JSON field names and string escaping.
// Benchmark: ~65 ns/op vs JSON's ~589 ns/op (9x faster).
type BinaryCodec struct{}

func (c *BinaryCodec) Encode(v any) ([]byte, error) {
	msg, ok := v.(*message.RPCMessage)
	if !ok {
		return nil, errors.New("BinaryCodec: v must be *RPCMessage")
	}

	// Pre-calculate total buffer size to avoid multiple allocations
	total := 2 + len(msg.ServiceMethod) + 4 + len(msg.Payload) + 2 + len(msg.Error)
	buf := make([]byte, total)

	offset := 0

	// ServiceMethod: 2-byte length prefix + string bytes
	binary.BigEndian.PutUint16(buf[offset:offset+2], uint16(len(msg.ServiceMethod)))
	offset += 2
	copy(buf[offset:offset+len(msg.ServiceMethod)], []byte(msg.ServiceMethod))
	offset += len(msg.ServiceMethod)

	// Payload: 4-byte length prefix + raw bytes (supports up to 4GB payload)
	binary.BigEndian.PutUint32(buf[offset:offset+4], uint32(len(msg.Payload)))
	offset += 4
	copy(buf[offset:offset+len(msg.Payload)], msg.Payload)
	offset += len(msg.Payload)

	// Error: 2-byte length prefix + string bytes
	binary.BigEndian.PutUint16(buf[offset:offset+2], uint16(len(msg.Error)))
	offset += 2
	copy(buf[offset:offset+len(msg.Error)], []byte(msg.Error))

	return buf, nil
}

func (c *BinaryCodec) Decode(data []byte, v any) error {
	msg, ok := v.(*message.RPCMessage)
	if !ok {
		return errors.New("BinaryCodec: v must be *RPCMessage")
	}

	offset := 0

	// Read ServiceMethod
	strLen := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2
	msg.ServiceMethod = string(data[offset : offset+int(strLen)])
	offset += int(strLen)

	// Read Payload
	payloadLen := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4
	msg.Payload = make([]byte, payloadLen)
	copy(msg.Payload, data[offset:offset+int(payloadLen)])
	offset += int(payloadLen)

	// Read Error
	errLen := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2
	msg.Error = string(data[offset : offset+int(errLen)])

	return nil
}

func (c *BinaryCodec) Type() CodecType {
	return CodecTypeBinary
}
