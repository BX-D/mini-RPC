package codec

import (
	"encoding/json"
)

// JSONCodec uses Go's standard library encoding/json for serialization.
// Pros: human-readable, cross-language, easy to debug.
// Cons: slower due to reflection + string parsing, larger payload (field names repeated).
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
