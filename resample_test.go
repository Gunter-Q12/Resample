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

	t.Run("New", func(t *testing.T) {
		_, err := ResampleInt16(input, ir, or, ch, quality)
		assert.NoError(t, err)
	})
	t.Run("in=out", func(t *testing.T) {
		output, err := ResampleInt16(input, 1, 1, ch, quality)
		assert.NoError(t, err)
		assert.Equal(t, input, output)
	})
	t.Run("not enough samples", func(t *testing.T) {
		_, err := ResampleInt16([]int16{1}, ir, or, ch, quality)
		assert.Error(t, err)
	})
}
