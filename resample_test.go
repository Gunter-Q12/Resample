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
		_, err := Int16(input, ir, or, ch, quality)
		assert.NoError(t, err)
	})
	t.Run("in=out", func(t *testing.T) {
		output, err := Int16(input, 1, 1, ch, quality)
		assert.NoError(t, err)
		assert.Equal(t, input, output)
	})
	t.Run("not enough samples", func(t *testing.T) {
		_, err := Int16([]int16{1}, ir, or, ch, quality)
		assert.Error(t, err)
	})
	t.Run("simplest case", func(t *testing.T) {
		want := []int16{1, 2, 3, 4, 5}
		got, err := Int16([]int16{1, 3, 5}, ir, or, ch, quality)

		assert.NoError(t, err)
		assert.Equal(t, want, got)
	})
}
