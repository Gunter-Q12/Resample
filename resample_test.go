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

	resamplerTestInt16 := []struct {
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
			input: []int16{1, 2, 3, 4, 5}, output: []int16{1, 3},
			err: nil, ir: 2, or: 1, q: Linear},
		{name: "kaiser_fast",
			input: []int16{1, 2, 3, 4, 5},
			err:   errors.New(""), ir: 2, or: 1, q: KaiserFast},
	}
	for _, tt := range resamplerTestInt16 {
		t.Run(tt.name, func(t *testing.T) {
			outBuf := new(bytes.Buffer)
			inBuf := writeBuff(t, tt.input)

			res, err := New[int16](outBuf, tt.ir, tt.or, ch, tt.q)
			assert.NoError(t, err)

			_, err = res.Write(inBuf.Bytes())
			if tt.err != nil {
				assert.Error(t, err)
			} else {
				output := readBuff[int16](t, outBuf, len(tt.output))
				assert.Equal(t, tt.output, output)
			}
		})
	}

	resamplerTestFloat64 := []struct {
		name   string
		input  []float64
		output []float64
		err    error
		ir     int
		or     int
		q      Quality
	}{
		{name: "real downsampling",
			input:  []float64{0, 0.25, 0.5, 0.75},
			output: []float64{0, 1.0 / 3, 2.0 / 3},
			err:    nil, ir: 4, or: 3, q: Linear},
		{name: "real upsampling",
			input:  []float64{1, 2, 3},
			output: []float64{1, 1.5, 2, 2.5, 3},
			err:    nil, ir: 2, or: 4, q: Linear},
	}
	for _, tt := range resamplerTestFloat64 {
		t.Run(tt.name, func(t *testing.T) {
			outBuf := new(bytes.Buffer)
			inBuf := writeBuff(t, tt.input)

			res, err := New[float64](outBuf, tt.ir, tt.or, ch, tt.q)
			assert.NoError(t, err)

			_, err = res.Write(inBuf.Bytes())
			if tt.err != nil {
				assert.Error(t, err)
			} else {
				output := readBuff[float64](t, outBuf, len(tt.output))
				assert.InDeltaSlicef(t, tt.output, output, .001, "output: %v", output)
			}
		})
	}

	t.Run("io.Copy", func(t *testing.T) {
		inBuf := writeBuff(t, []int16{1, 2, 3})
		outBuf := new(bytes.Buffer)

		res, err := New[int16](outBuf, 1, 2, ch, Linear)
		assert.NoError(t, err)

		size, err := io.Copy(res, inBuf)
		assert.NoError(t, err)
		assert.EqualValues(t, 6, size)
	})
}

func TestGetSincWindow(t *testing.T) {
	path := "./testdata/sinc_window_"
	zeros := 16
	density := 8

	file, err := os.OpenFile(path+"want", os.O_RDONLY, 0666)
	if errors.Is(err, os.ErrNotExist) {
		want, err := getSincWindow(zeros, density)
		assert.NoError(t, err)
		toFile(t, want, path+"got")
		t.Fatalf("Check saved results.\nRename file form *_got to *_want\nRun the test again")
	} else if err != nil {
		t.Fatal(err)
	}
	want := readBuff[float64](t, file, zeros*density+1)

	got, err := getSincWindow(zeros, density)
	assert.NoError(t, err)

	toFile(t, got, path+"got")
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

func toFile(t *testing.T, values any, path string) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	err = binary.Write(file, binary.LittleEndian, values)
	if err != nil {
		t.Fatal(err)
	}
}

func writeBuff(t *testing.T, values any) *bytes.Buffer {
	inBuf := new(bytes.Buffer)
	err := binary.Write(inBuf, binary.LittleEndian, values)
	if err != nil {
		t.Fatal(err)
	}
	return inBuf
}

func readBuff[T any](t *testing.T, buff io.Reader, len int) []T {
	output := make([]T, len)

	err := binary.Read(buff, binary.LittleEndian, output)
	assert.NoError(t, err)

	return output
}
