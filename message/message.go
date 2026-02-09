package message

type RPCMessage struct {
	ServiceMethod string
	Error         string
	Payload       []byte
}
