package broker

import (
	"testing"
)

func BenchmarkJsonMarshaler_Marshal_Struct(b *testing.B) {
	m := &JsonMarshaler{}
	data := struct {
		Name  string
		Value int
	}{
		Name:  "test",
		Value: 123,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Marshal(data)
	}
}

func BenchmarkJsonMarshaler_Marshal_Bytes(b *testing.B) {
	m := &JsonMarshaler{}
	data := []byte("hello world")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Marshal(data)
	}
}

func BenchmarkJsonMarshaler_Marshal_String(b *testing.B) {
	m := &JsonMarshaler{}
	data := "hello world"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Marshal(data)
	}
}
