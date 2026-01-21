package broker

import (
	"bytes"
	"encoding/json"
	"sync"
)

var (
	bufferPool = sync.Pool{
		New: func() any {
			return new(bytes.Buffer)
		},
	}
)

type JsonMarshaler struct{}

func (j JsonMarshaler) Marshal(v any) ([]byte, error) {
	switch d := v.(type) {
	case []byte:
		return d, nil
	case string:
		return []byte(d), nil
	default:
		buf := bufferPool.Get().(*bytes.Buffer)
		buf.Reset()
		defer bufferPool.Put(buf)

		if err := json.NewEncoder(buf).Encode(v); err != nil {
			return nil, err
		}
		// json.Encoder adds a newline at the end, which we usually don't want for MQ messages
		res := buf.Bytes()
		if len(res) > 0 && res[len(res)-1] == '\n' {
			res = res[:len(res)-1]
		}

		// We must return a copy because buf is reused
		out := make([]byte, len(res))
		copy(out, res)
		return out, nil
	}
}

func (j JsonMarshaler) Unmarshal(d []byte, v any) error {
	return json.Unmarshal(d, v)
}

func (j JsonMarshaler) String() string {
	return "json"
}
