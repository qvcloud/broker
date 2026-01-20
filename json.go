package broker

import (
	"encoding/json"
)

type JsonMarshaler struct{}

func (j JsonMarshaler) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (j JsonMarshaler) Unmarshal(d []byte, v any) error {
	return json.Unmarshal(d, v)
}

func (j JsonMarshaler) String() string {
	return "json"
}
