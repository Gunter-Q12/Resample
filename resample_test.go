package resample

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
	quality := Linear
	output := io.Discard

	res, err := New(output, ir, or, ch, frmt, quality)
	assert.NoError(t, err)

	data := []int16{1}
	_, err = res.Write(data)
	assert.Error(t, err)

}
