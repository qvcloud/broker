package broker

import (
	"context"
	"encoding/json"
	"testing"
)

type complexStruct struct {
	ID     int      `json:"id"`
	Name   string   `json:"name"`
	Tags   []string `json:"tags"`
	Active bool     `json:"active"`
}

func BenchmarkJsonMarshaler_SmartSerialization(b *testing.B) {
	marshaler := JsonMarshaler{}
	dataBytes := []byte("hello world this is a test payload")
	dataString := "hello world this is a test payload"
	dataStruct := complexStruct{
		ID:     123,
		Name:   "Test Object",
		Tags:   []string{"tag1", "tag2", "tag3"},
		Active: true,
	}

	b.Run("Bytes", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = marshaler.Marshal(dataBytes)
		}
	})

	b.Run("String", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = marshaler.Marshal(dataString)
		}
	})

	b.Run("Struct", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = marshaler.Marshal(dataStruct)
		}
	})
}

func BenchmarkStandardJson_Marshal(b *testing.B) {
	dataBytes := []byte("hello world this is a test payload")
	dataString := "hello world this is a test payload"

	b.Run("Bytes", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = json.Marshal(dataBytes)
		}
	})

	b.Run("String", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = json.Marshal(dataString)
		}
	})
}

func BenchmarkStandardJson_Marshal_Struct(b *testing.B) {
	dataStruct := complexStruct{
		ID:     123,
		Name:   "Test Object",
		Tags:   []string{"tag1", "tag2", "tag3"},
		Active: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(dataStruct)
	}
}

func BenchmarkOptionTracker(b *testing.B) {
	ctx := context.Background()

	b.Run("WithTrackedValue", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = WithTrackedValue(ctx, "key", "val", "test.Option")
		}
	})

	b.Run("GetTrackedValue", func(b *testing.B) {
		trackedCtx := WithTrackedValue(ctx, "key", "val", "test.Option")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = GetTrackedValue(trackedCtx, "key")
		}
	})

	b.Run("StandardContextValue", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = context.WithValue(ctx, "key", "val")
		}
	})
}
