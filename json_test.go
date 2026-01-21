package broker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJsonMarshaler_Marshal(t *testing.T) {
	m := JsonMarshaler{}

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

func TestJsonMarshaler_Unmarshal(t *testing.T) {
	m := JsonMarshaler{}

	t.Run("Struct", func(t *testing.T) {
		type data struct{ Name string }
		input := []byte(`{"Name":"test"}`)
		var output data
		err := m.Unmarshal(input, &output)
		assert.NoError(t, err)
		assert.Equal(t, "test", output.Name)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		input := []byte(`{invalid}`)
		var output map[string]any
		err := m.Unmarshal(input, &output)
		assert.Error(t, err)
	})
}

func TestJsonMarshaler_String(t *testing.T) {
	m := JsonMarshaler{}
	assert.Equal(t, "json", m.String())
}
