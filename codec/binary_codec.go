package codec

import (
	"encoding/binary"
	"errors"
	"mini-rpc/message"
)

type BinaryCodec struct{}

func (c *BinaryCodec) Encode(v any) ([]byte, error) {
	// v must be *RPCMessage
	msg, ok := v.(*message.RPCMessage)
	if !ok {
		return nil, errors.New("BinaryCodec: v must be *RPCMessage")
	}
	// Caculate the length of message
	total := 2 + len(msg.ServiceMethod) + 4 + len(msg.Payload) + 2 + len(msg.Error)
	buf := make([]byte, total)

	offset := 0
	// ServiceMethod length -- 2 bytes
	binary.BigEndian.PutUint16(buf[offset:offset+2], uint16(len(msg.ServiceMethod)))
	offset += 2

	// ServiceMethod -- n bytes
	copy(buf[offset:offset+len(msg.ServiceMethod)], []byte(msg.ServiceMethod))
	offset += len(msg.ServiceMethod)

	// Payload length -- 4 bytes
	binary.BigEndian.PutUint32(buf[offset:offset+4], uint32(len(msg.Payload)))
	offset += 4

	// Payload -- n bytes
	copy(buf[offset:offset+len(msg.Payload)], msg.Payload)
	offset += len(msg.Payload)

	// Error length -- 2 bytes
	binary.BigEndian.PutUint16(buf[offset:offset+2], uint16(len(msg.Error)))
	offset += 2

	// Error -- n bytes
	copy(buf[offset:offset+len(msg.Error)], []byte(msg.Error))
	return buf, nil
}

func (c *BinaryCodec) Decode(data []byte, v any) error {
	// v must be *RPCMessage
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
