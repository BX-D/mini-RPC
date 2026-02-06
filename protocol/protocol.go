package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	MagicNumber byte = 0x6d // 'm'
	MagicByte2  byte = 0x72 // 'r'
	MagicByte3  byte = 0x70 // 'p'
	Version     byte = 0x01
	HeaderSize  int  = 14 // 3+1+1+1+4+4 = 14 bytes
)

type MsgType byte

const (
	MsgTypeRequest   MsgType = 0
	MsgTypeResponse  MsgType = 1
	MsgTypeHeartbeat MsgType = 2
)

const (
    CodecTypeJSON   byte = 0
    CodecTypeBinary byte = 1
)

type Header struct {
	CodecType byte // 0=JSON, 1=Binary
	MsgType   MsgType
	Seq       uint32
	BodyLen   uint32
}

// Encode 将帧头写入 writer
func Encode(w io.Writer, h *Header, body []byte) error {
	// Create 15 byte buffer
	buf := make([]byte, HeaderSize)
	// Magic Number -- 3 bytes
	copy(buf[0:3], []byte{MagicNumber, MagicByte2, MagicByte3})
	// Version -- 1 byte
	buf[3] = Version
	// Codec Type -- 1 byte
	buf[4] = h.CodecType
	// Msg Type -- 1 byte
	buf[5] = byte(h.MsgType)
	// Sequence Number -- 4 bytes
	binary.BigEndian.PutUint32(buf[6:10], h.Seq)
	// Body Length -- 4 bytes
	binary.BigEndian.PutUint32(buf[10:14], h.BodyLen)
	// Write header and body to writer
	if _, err := w.Write(buf); err != nil {
		return err
	}
	
	if _, err := w.Write(body); err != nil {
		return err
	}
	return nil
}

// Decode 从 reader 读取帧头和消息体
func Decode(r io.Reader) (*Header, []byte, error) {
    // 读 14 字节帧头
	headerBuf := make([]byte, HeaderSize)
    // 校验 magic
	if _, err := io.ReadFull(r, headerBuf); err != nil {
		return nil, nil, err
	}
	// Decode magic number
	if headerBuf[0] != MagicNumber || headerBuf[1] != MagicByte2 || headerBuf[2] != MagicByte3 {
		return nil, nil, fmt.Errorf("invalid magic number: %x", headerBuf[0:3])
	}
    // Decode version
	if headerBuf[3] != Version {
		return nil, nil, fmt.Errorf("unsupported version: %d", headerBuf[3])
	}
	// Decode codec type
	if headerBuf[4] != CodecTypeJSON && headerBuf[4] != CodecTypeBinary {
		return nil, nil, fmt.Errorf("unsupported codec type: %d", headerBuf[4])
	}
    // Decode msg type
	msgType := headerBuf[5]
	if msgType != byte(MsgTypeRequest) && msgType != byte(MsgTypeResponse) && msgType != byte(MsgTypeHeartbeat) {
		return nil, nil, fmt.Errorf("unsupported message type: %d", msgType)
	}
	
	// Decode sequence number
	seq := binary.BigEndian.Uint32(headerBuf[6:10])
	// Decode body length
	bodyLen := binary.BigEndian.Uint32(headerBuf[10:14])

	// read body
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
