package proto

import (
	"encoding/json"

	"google.golang.org/grpc/encoding"
)

// jsonCodec is a simple JSON codec so our hand-written proto structs can be used with gRPC in this demo.
// In production you should use real protobuf-generated types and the protobuf codec.
type jsonCodec struct{}

func (jsonCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
func (jsonCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
func (jsonCodec) Name() string { return "json" }

func init() {
	encoding.RegisterCodec(jsonCodec{})
}
