package codec

type CodecType byte

const (
	CodecTypeJSON   CodecType = 0
	CodecTypeBinary CodecType = 1
)

type Codec interface {
	Encode(v any) ([]byte, error)
	Decode(data []byte, v any) error
	Type() CodecType // 0=JSON, 1=Binary
}

func GetCodec(codecType CodecType) Codec {
	if codecType == CodecTypeJSON {
		return &JSONCodec{}
	}

	return &BinaryCodec{}
}
