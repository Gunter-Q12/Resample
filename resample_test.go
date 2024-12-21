package resample

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
)

func TestResampler(t *testing.T) {
	ch := 1
	format := I16
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
		{name: "not enough samples",
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
			outBuf := new(bytes.Buffer)

			inBuf := new(bytes.Buffer)
			err := binary.Write(inBuf, binary.LittleEndian, tt.input)
			assert.NoError(t, err)

			res, err := New(outBuf, tt.ir, tt.or, ch, format, quality)
			assert.NoError(t, err)

			_, err = res.Write(inBuf.Bytes())
			if tt.err != nil {
				assert.Error(t, err)
			}
			if tt.err == nil {
				output := make([]int16, len(tt.output))
				err := binary.Read(outBuf, binary.LittleEndian, output)

				assert.NoError(t, err)
				assert.Equal(t, tt.output, output)
			}
		})
	}

	t.Run("io.Copy", func(t *testing.T) {
		inBuf := new(bytes.Buffer)
		err := binary.Write(inBuf, binary.LittleEndian, []int16{1, 3, 5})
		assert.NoError(t, err)

		outBuf := new(bytes.Buffer)
		res, err := New(outBuf, 1, 2, ch, format, quality)
		assert.NoError(t, err)

		size, err := io.Copy(res, inBuf)
		assert.NoError(t, err)
		assert.EqualValues(t, 6, size)
	})
}

func FuzzResampler(f *testing.F) {
	f.Fuzz(func(t *testing.T, samples []byte, ir, or int) {
		if len(samples)%2 == 0 {
			return
		}

		res, err := New(io.Discard, ir, or, 1, I16, Linear)
		if err != nil {
			return
		}
		_, _ = res.Write(samples)
	})
}
