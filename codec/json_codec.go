package codec

import (
	"encoding/json"
)

type JSONCodec struct{}

func (c *JSONCodec) Encode(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (c *JSONCodec) Decode(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (c *JSONCodec) Type() CodecType {
	return CodecTypeJSON
}
