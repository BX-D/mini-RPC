package protocol

import (
	"testing"
	"bytes"	
)


func TestEncodeDecode(t *testing.T) {
	// Prepare header and body
	header := Header{
		CodecType: CodecTypeJSON,
		MsgType:   MsgTypeRequest,
		Seq:       12345,
		BodyLen: 11,
	}
	body := []byte("hello world")

	// Encode header and body into buffer
	var buf bytes.Buffer
	if err := Encode(&buf, &header, body); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode header and body from buffer
	decodedHeader, decodedBody, err := Decode(&buf)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify decoded header
	if decodedHeader.CodecType != header.CodecType {
		t.Errorf("CodecType mismatch: got %d, want %d", decodedHeader.CodecType, header.CodecType)
	}
	if decodedHeader.MsgType != header.MsgType {
		t.Errorf("MsgType mismatch: got %d, want %d", decodedHeader.MsgType, header.MsgType)
	}
	if decodedHeader.Seq != header.Seq {
		t.Errorf("Seq mismatch: got %d, want %d", decodedHeader.Seq, header.Seq)
	}
	if decodedHeader.BodyLen != header.BodyLen {
		t.Errorf("BodyLen mismatch: got %d, want %d", decodedHeader.BodyLen, header.BodyLen)
	}

	// Verify decoded body
	if !bytes.Equal(decodedBody, body) {
		t.Errorf("Body mismatch: got %s, want %s", string(decodedBody), string(body))
	}

	t.Logf("Pass all the test for Encode and Decode!")
}

func TestDecodeInvalidMagic(t *testing.T) {
	// Prepare invalid header with wrong magic number
	invalidHeader := []byte{0x00, 0x00, 0x00, Version, CodecTypeJSON, byte(MsgTypeRequest), 0x00, 0x00, 0x30, 0x39, 0x00, 0x00, 0x00, 0x0B}
	var buf bytes.Buffer
	buf.Write(invalidHeader)
	buf.Write([]byte("hello world"))

	// Decode should fail with invalid magic number error
	_, _, err := Decode(&buf)
	if err == nil {
		t.Fatal("Expected error for invalid magic number, but got nil")
	}

	if !bytes.Contains([]byte(err.Error()), []byte("invalid magic number")) {
		t.Errorf("Error message should contain 'invalid magic', instead: %v", err)
	}

	t.Logf("Pass the test for invalid magic number!")
}

func TestDecodeEmptyBody(t *testing.T) {
	// Prepare header with zero body length
	header := Header{
		CodecType: CodecTypeJSON,
		MsgType:   MsgTypeHeartbeat,
		Seq:       12345,
		BodyLen: 0,
	}
	var buf bytes.Buffer
	if err := Encode(&buf, &header, []byte{}); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode should succeed with empty body
	decodedHeader, decodedBody, err := Decode(&buf)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decodedHeader.MsgType != MsgTypeHeartbeat {
		t.Errorf("MsgType mismatch: got %d, want %d", decodedHeader.MsgType, MsgTypeHeartbeat)
	}
	if decodedHeader.BodyLen != 0 {
		t.Errorf("BodyLen mismatch: got %d, want 0", decodedHeader.BodyLen)
	}
	if len(decodedBody) != 0 {
		t.Errorf("Expected empty body, got length %d", len(decodedBody))
	}

	t.Logf("Pass the test for empty body!")
}

func TestDecodeInvalidVersion(t *testing.T) {
    var buf bytes.Buffer
    
    // 手动构造错误 Version 的帧
    invalidFrame := []byte{
        MagicNumber, MagicByte2, MagicByte3, // 正确的 Magic
        0xFF,        // 错误的 Version
        CodecTypeJSON,
        byte(MsgTypeRequest),
        0, 0, 0, 1,  // Seq
        0, 0, 0, 0,  // BodyLen
    }
    buf.Write(invalidFrame)
    
    _, _, err := Decode(&buf)
    if err == nil {
        t.Fatal("期待返回错误，但 Decode 成功了")
    }
    
    if !bytes.Contains([]byte(err.Error()), []byte("unsupported version")) {
        t.Errorf("错误信息应该包含 'unsupported version', 实际: %v", err)
    }
    
    t.Logf("✅ 正确识别了错误的 Version: %v", err)
}

func TestDecodeLargeBody(t *testing.T) {
    var buf bytes.Buffer
    
    // 1MB 的消息体
    largeBody := make([]byte, 1024*1024)
    for i := range largeBody {
        largeBody[i] = byte(i % 256)
    }
    
    header := &Header{
        CodecType: CodecTypeBinary,
        MsgType:   MsgTypeRequest,
        Seq:       999,
        BodyLen:   uint32(len(largeBody)),
    }
    
    // 编码
    if err := Encode(&buf, header, largeBody); err != nil {
        t.Fatalf("Encode 失败: %v", err)
    }
    
    // 解码
    _, decodedBody, err := Decode(&buf)
    if err != nil {
        t.Fatalf("Decode 失败: %v", err)
    }
    
    // 验证
    if !bytes.Equal(decodedBody, largeBody) {
        t.Errorf("大消息体内容不匹配")
    }
    
    t.Logf("✅ 成功编解码 %d 字节的大消息体", len(largeBody))
}