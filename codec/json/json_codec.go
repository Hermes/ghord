package json

import (
	"encoding/json"
	"io"
)

type Codec struct{}

func NewCodec() JSONCodec {
	return JSONCodec{}
}

func (c Codec) Name() string {
	return "json"
}

func (c Codec) NewEncoder(w io.Writer) *json.Encoder {
	return json.NewEncoder(w)
}

func (c Codec) NewDecoder(r io.Reader) *json.Decoder {
	return json.NewDecoder(r)
}

func (c Codec) Encode(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (c Codec) Encode(data interface{}, v interface{}) error {
	return json.Unmarshal(data, v)
}
