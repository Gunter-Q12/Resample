package pkg

import (
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
)

func TestResampler(t *testing.T) {
	ir := 8000.0
	or := 16000.0
	ch := 1
	frmt := I16
	quality := -1
	output := io.Discard

	_, err := New(output, ir, or, ch, frmt, quality)
	assert.NoError(t, err)

}
