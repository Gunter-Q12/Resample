package resample

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"testing"
)

func TestResampler(t *testing.T) {
	ch := 1

	resamplerTest := []struct {
		name   string
		input  []int16
		output []int16
		err    error
		ir     int
		or     int
		q      Quality
	}{
		{name: "in=out",
			input: []int16{1, 2, 3}, output: []int16{1, 2, 3},
			err: nil, ir: 1, or: 1, q: Linear},
		{name: "not enough samples",
			input: []int16{1},
			err:   errors.New(""), ir: 1, or: 2, q: Linear},
		{name: "simplest upsampling case",
			input: []int16{1, 3, 5}, output: []int16{1, 2, 3, 4, 5},
			err: nil, ir: 1, or: 2, q: Linear},
		{name: "simplest downsampling case",
			input: []int16{1, 2, 3, 4, 5}, output: []int16{1, 3, 5},
			err: nil, ir: 2, or: 1, q: Linear},
		{name: "kaiser_fast",
			input: []int16{1, 2, 3, 4, 5},
			err:   errors.New(""), ir: 2, or: 1, q: KaiserFast},
	}
	for _, tt := range resamplerTest {
		t.Run(tt.name, func(t *testing.T) {
			outBuf := new(bytes.Buffer)

			inBuf := new(bytes.Buffer)
			err := binary.Write(inBuf, binary.LittleEndian, tt.input)
			assert.NoError(t, err)

			res, err := New[int16](outBuf, tt.ir, tt.or, ch, tt.q)
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
		res, err := New[int16](outBuf, 1, 2, ch, Linear)
		assert.NoError(t, err)

		size, err := io.Copy(res, inBuf)
		assert.NoError(t, err)
		assert.EqualValues(t, 6, size)
	})

	t.Run("float64", func(t *testing.T) {
		outBuf := new(bytes.Buffer)

		inBuf := new(bytes.Buffer)
		err := binary.Write(inBuf, binary.LittleEndian, []float64{1, 2, 3})
		assert.NoError(t, err)

		res, err := New[float64](outBuf, 1, 2, ch, Linear)
		assert.NoError(t, err)

		_, err = res.Write(inBuf.Bytes())
		assert.NoError(t, err)

		output := make([]float64, 5)
		err = binary.Read(outBuf, binary.LittleEndian, output)
		assert.NoError(t, err)
		assert.Equal(t, []float64{1, 1.5, 2, 2.5, 3}, output)
	})
}

func TestGetSincWindow(t *testing.T) {
	raw, err := os.ReadFile("./testdata/sinc_window")
	assert.NoError(t, err)

	want := make([]float64, len(raw)/8)
	err = binary.Read(bytes.NewReader(raw), binary.LittleEndian, &want)
	assert.NoError(t, err)

	got, err := getSincWindow(64, 9, 0.945)
	//toFile(got, "testdata/sinc_window_go")
	assert.NoError(t, err)
	assert.Lenf(t, got, len(want), "want: %d, got: %d", len(want), len(got))
	assert.InDeltaSlice(t, want, got, 0.0001)

}

func FuzzResampler(f *testing.F) {
	f.Fuzz(func(t *testing.T, samples []byte, ir, or int) {
		if len(samples)%2 != 0 {
			return
		}

		res, err := New[int16](io.Discard, ir, or, 1, Linear)
		if err != nil {
			return
		}
		_, _ = res.Write(samples)
	})
}

func toFile(values any, path string) {
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	err = binary.Write(file, binary.LittleEndian, values)
	if err != nil {
		panic(err)
	}
}
