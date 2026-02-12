package client

import (
	"encoding/json"
	"fmt"
	"mini-rpc/codec"
	"mini-rpc/message"
	"mini-rpc/protocol"
	"net"
)

type Client struct {
	conn      net.Conn
	codecType codec.CodecType
	seq       uint32
}

func NewClient(conn net.Conn, codecType byte) *Client {
	return &Client{
		conn:      conn,
		codecType: codec.CodecType(codecType),
	}
}

func (c *Client) Call(serviceMethod string, args any, reply any) error {
	c.seq++
	// Marshal args to json
	payload, err := json.Marshal(args)
	if err != nil {
		return err
	}
	// Create a RPCMessage
	rpcMessage := message.RPCMessage{
		ServiceMethod: serviceMethod,
		Error:         "",
		Payload:       payload,
	}

	// Get the codec
	cdc := codec.GetCodec(c.codecType)

	// Encode the message
	body, err := cdc.Encode(&rpcMessage)

	if err != nil {
		return err
	}

	// Construct the header
	header := protocol.Header{
		CodecType: byte(c.codecType),
		MsgType:   protocol.MsgTypeRequest,
		Seq:       c.seq,
		BodyLen:   uint32(len(body)),
	}

	// Send the request
	err = protocol.Encode(c.conn, &header, body)

	if err != nil {
		return err
	}

	// Wait for the response
	replyHeader, responseBody, err := protocol.Decode(c.conn)

	if err != nil {
		return err
	}

	if replyHeader.Seq != header.Seq {
		return fmt.Errorf("expect replyHeader with seq: %v, get %v", header.Seq, replyHeader.Seq)
	}

	if replyHeader.CodecType != header.CodecType {
		return fmt.Errorf("expect replyHeader with CodecType: %v, get %v", header.CodecType, replyHeader.CodecType)
	}

	if replyHeader.MsgType != protocol.MsgTypeResponse {
		return fmt.Errorf("expect replyHeader with MsgType: %v, get %v", header.MsgType, replyHeader.MsgType)
	}

	// Decode the response body to RPCMessage
	responseRPC := message.RPCMessage{}

	err = cdc.Decode(responseBody, &responseRPC)

	if err != nil {
		return err
	}

	if responseRPC.Error != "" {
		return fmt.Errorf("server error: %v", responseRPC.Error)
	}

	// Unmarshal the payload to reply
	err = json.Unmarshal(responseRPC.Payload, &reply)

	if err != nil {
		return err
	}

	return nil
}
