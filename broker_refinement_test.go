package broker_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/qvcloud/broker"
	"github.com/stretchr/testify/assert"
)

type mockLogger struct {
	warnings []string
}

func (l *mockLogger) Log(v ...any) {}
func (l *mockLogger) Logf(format string, v ...any) {
	l.warnings = append(l.warnings, fmt.Sprintf(format, v...))
}

func TestOptionTracker(t *testing.T) {
	logger := &mockLogger{}
	ctx := context.Background()

	type testKey struct{}

	// Register an option
	ctx = broker.WithTrackedValue(ctx, testKey{}, "val", "test.WithOption")

	// Check before consumption
	broker.WarnUnconsumed(ctx, logger)
	assert.Len(t, logger.warnings, 1)
	assert.Contains(t, logger.warnings[0], "test.WithOption")

	// Reset warnings
	logger.warnings = nil

	// Consume
	val := broker.GetTrackedValue(ctx, testKey{})
	assert.Equal(t, "val", val)

	// Check after consumption
	broker.WarnUnconsumed(ctx, logger)
	assert.Len(t, logger.warnings, 0)
}

func TestJsonMarshaler_SmartSerialization(t *testing.T) {
	m := broker.JsonMarshaler{}

	t.Run("RawBytes", func(t *testing.T) {
		input := []byte("hello world")
		output, err := m.Marshal(input)
		assert.NoError(t, err)
		assert.Equal(t, input, output, "Should be zero-copy for []byte")
	})

	t.Run("String", func(t *testing.T) {
		input := "hello world"
		output, err := m.Marshal(input)
		assert.NoError(t, err)
		assert.Equal(t, []byte(input), output)
	})

	t.Run("Struct", func(t *testing.T) {
		type data struct{ Name string }
		input := data{Name: "test"}
		output, err := m.Marshal(input)
		assert.NoError(t, err)
		assert.JSONEq(t, `{"Name":"test"}`, string(output))
	})
}
