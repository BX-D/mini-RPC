// Package protocol implements the custom binary frame protocol for mini-RPC.
//
// It solves TCP's sticky packet problem by using a fixed-size 14-byte header
// followed by a variable-length body. The receiver reads the header first to
// determine the body length, then reads exactly that many bytes.
//
// Frame format:
//
//	0      3  4  5  6         10        14
//	┌──────┬──┬──┬──┬─────────┬─────────┬───────────────┐
//	│magic │v │ct│mt│   seq   │ bodyLen │    body ...    │
//	│ mrp  │01│  │  │ uint32  │ uint32  │ bodyLen bytes  │
//	└──────┴──┴──┴──┴─────────┴─────────┴───────────────┘
package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Magic number bytes: "mrp" (mini-rpc protocol).
// Used to quickly identify whether the incoming data is a valid mini-RPC frame,
// rejecting non-protocol connections (e.g., HTTP clients hitting the wrong port).
const (
	MagicNumber byte = 0x6d // 'm'
	MagicByte2  byte = 0x72 // 'r'
	MagicByte3  byte = 0x70 // 'p'
	Version     byte = 0x01
	HeaderSize  int  = 14 // 3 (magic) + 1 (version) + 1 (codec) + 1 (msgType) + 4 (seq) + 4 (bodyLen)
)

// MsgType distinguishes request, response, and heartbeat frames.
type MsgType byte

const (
	MsgTypeRequest   MsgType = 0 // Client → Server RPC request
	MsgTypeResponse  MsgType = 1 // Server → Client RPC response
	MsgTypeHeartbeat MsgType = 2 // KeepAlive probe (no body)
)

// Codec type constants, mirrored from codec package to avoid circular import.
const (
	CodecTypeJSON   byte = 0
	CodecTypeBinary byte = 1
)

// Header represents the fixed 14-byte frame header.
// It carries metadata needed to decode the following body correctly.
type Header struct {
	CodecType byte    // Serialization format: 0=JSON, 1=Binary
	MsgType   MsgType // Request, Response, or Heartbeat
	Seq       uint32  // Sequence ID — the key to multiplexing (matches request ↔ response)
	BodyLen   uint32  // Body length in bytes — solves TCP sticky packet problem
}

// Encode writes a complete frame (header + body) to w.
// The caller must hold a write lock if multiple goroutines share the same writer,
// otherwise frames from different requests will interleave and corrupt the stream.
func Encode(w io.Writer, h *Header, body []byte) error {
	buf := make([]byte, HeaderSize)

	// Magic number: 3 bytes — protocol identification
	copy(buf[0:3], []byte{MagicNumber, MagicByte2, MagicByte3})
	// Version: 1 byte — for future protocol upgrades
	buf[3] = Version
	// Codec type: 1 byte
	buf[4] = h.CodecType
	// Message type: 1 byte
	buf[5] = byte(h.MsgType)
	// Sequence number: 4 bytes, big-endian (network byte order)
	binary.BigEndian.PutUint32(buf[6:10], h.Seq)
	// Body length: 4 bytes, big-endian
	binary.BigEndian.PutUint32(buf[10:14], h.BodyLen)

	// Write header
	if _, err := w.Write(buf); err != nil {
		return err
	}
	// Write body (may be nil for heartbeat frames)
	if _, err := w.Write(body); err != nil {
		return err
	}
	return nil
}

// Decode reads a complete frame (header + body) from r.
// It validates the magic number, version, codec type, and message type.
// Uses io.ReadFull to guarantee exactly N bytes are read, preventing partial reads.
func Decode(r io.Reader) (*Header, []byte, error) {
	// Step 1: Read the fixed 14-byte header
	headerBuf := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, headerBuf); err != nil {
		return nil, nil, err
	}

	// Step 2: Validate magic number — reject non-protocol connections
	if headerBuf[0] != MagicNumber || headerBuf[1] != MagicByte2 || headerBuf[2] != MagicByte3 {
		return nil, nil, fmt.Errorf("invalid magic number: %x", headerBuf[0:3])
	}

	// Step 3: Validate version
	if headerBuf[3] != Version {
		return nil, nil, fmt.Errorf("unsupported version: %d", headerBuf[3])
	}

	// Step 4: Validate codec type
	if headerBuf[4] != CodecTypeJSON && headerBuf[4] != CodecTypeBinary {
		return nil, nil, fmt.Errorf("unsupported codec type: %d", headerBuf[4])
	}

	// Step 5: Validate message type
	msgType := headerBuf[5]
	if msgType != byte(MsgTypeRequest) && msgType != byte(MsgTypeResponse) && msgType != byte(MsgTypeHeartbeat) {
		return nil, nil, fmt.Errorf("unsupported message type: %d", msgType)
	}

	// Step 6: Parse sequence number and body length
	seq := binary.BigEndian.Uint32(headerBuf[6:10])
	bodyLen := binary.BigEndian.Uint32(headerBuf[10:14])

	// Step 7: Read exactly bodyLen bytes — this is how we solve TCP sticky packet
	body := make([]byte, bodyLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, nil, err
	}

	return &Header{
		CodecType: headerBuf[4],
		MsgType:   MsgType(msgType),
		Seq:       seq,
		BodyLen:   bodyLen,
	}, body, nil
}
