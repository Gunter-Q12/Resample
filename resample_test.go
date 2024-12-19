package resample

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestResampler(t *testing.T) {
	input := []int16{1, 2, 3}
	ir := 8000
	or := 16000
	ch := 1
	quality := Linear

	_, err := ResampleInt16(input, ir, or, ch, quality)
	assert.NoError(t, err)
}
