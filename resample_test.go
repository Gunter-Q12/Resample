package resample

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestResampler(t *testing.T) {
	ch := 1
	quality := Linear

	resamplerTest := []struct {
		name   string
		input  []int16
		output []int16
		err    error
		ir     int
		or     int
	}{
		{name: "in=out",
			input: []int16{1, 2, 3}, output: []int16{1, 2, 3},
			err: nil, ir: 1, or: 1},
		{name: "in=out",
			input: []int16{1},
			err:   errors.New(""), ir: 1, or: 2},
		{name: "simplest upsampling case",
			input: []int16{1, 3, 5}, output: []int16{1, 2, 3, 4, 5},
			err: nil, ir: 1, or: 2},
		{name: "simplest downsampling case",
			input: []int16{1, 2, 3, 4, 5}, output: []int16{1, 3, 5},
			err: nil, ir: 2, or: 1},
	}
	for _, tt := range resamplerTest {
		t.Run(tt.name, func(t *testing.T) {
			output, err := Int16(tt.input, tt.ir, tt.or, ch, quality)
			if tt.err != nil {
				assert.Error(t, err)
			}
			if tt.err == nil {
				assert.Equal(t, tt.output, output)
			}
		})
	}
}
