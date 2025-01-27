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

func TestResamplerInt(t *testing.T) {
	resamplerTestInt16 := []struct {
		name   string
		input  []int16
		output []int16
		err    error
		ir     int
		or     int
		ch     int
		filter Option[int16]
	}{
		{name: "in=out",
			input: []int16{1, 2, 3}, output: []int16{1, 2, 3},
			err: nil, ir: 1, or: 1, ch: 1, filter: LinearFilter[int16]()},
		{name: "not enough samples",
			input: []int16{1},
			err:   errors.New(""), ir: 1, or: 2, ch: 1, filter: LinearFilter[int16]()},
		{name: "simplest upsampling case",
			input: []int16{1, 3, 5}, output: []int16{1, 2, 3, 4, 5},
			err: nil, ir: 1, or: 2, ch: 1, filter: LinearFilter[int16]()},
		{name: "simplest downsampling case",
			input: []int16{1, 2, 3, 4, 5}, output: []int16{1, 3},
			err: nil, ir: 2, or: 1, ch: 1, filter: LinearFilter[int16]()},
		{name: "two channels",
			input:  []int16{1, 11, 3, 13, 5, 15},
			output: []int16{1, 11, 2, 12, 3, 13, 4, 14, 5, 15},
			err:    nil, ir: 1, or: 2, ch: 2, filter: LinearFilter[int16]()},
	}
	for _, tt := range resamplerTestInt16 {
		t.Run(tt.name, func(t *testing.T) {
			outBuf := new(bytes.Buffer)
			inBuf := writeBuff(t, tt.input)

			res, err := New[int16](outBuf, tt.ir, tt.or, tt.ch, tt.filter)
			assert.NoError(t, err)

			_, err = res.Write(inBuf.Bytes())
			if tt.err != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				output := readBuff[int16](t, outBuf, len(tt.output))
				assert.Equal(t, tt.output, output)
			}
		})
	}

	t.Run("io.Copy", func(t *testing.T) {
		inBuf := writeBuff(t, []int16{1, 2, 3})
		outBuf := new(bytes.Buffer)

		res, err := New[int16](outBuf, 1, 2, 1, LinearFilter[int16]())
		assert.NoError(t, err)

		size, err := io.Copy(res, inBuf)
		assert.NoError(t, err)
		assert.EqualValues(t, 6, size)
	})
}

func TestResamplerFloat(t *testing.T) {
	ch := 1

	file, err := os.Open("./testdata/sine_8000_3_f64_ch1")
	if err != nil {
		t.Fatal(err)
	}
	sine8000 := readBuff[float64](t, file, 8000*3)

	file, err = os.Open("./testdata/sine_125_3_f64_ch1")
	if err != nil {
		t.Fatal(err)
	}
	sine125 := readBuff[float64](t, file, 125*3)

	resamplerTestFloat64 := []struct {
		name   string
		input  []float64
		output []float64
		err    error
		ir     int
		or     int
		filter Option[float64]
	}{
		{name: "Linear downsampling",
			input:  []float64{0, 0.25, 0.5, 0.75},
			output: []float64{0, 1.0 / 3, 2.0 / 3},
			err:    nil, ir: 4, or: 3, filter: LinearFilter[float64]()},
		{name: "Linear upsampling",
			input:  []float64{1, 2, 3},
			output: []float64{1, 1.5, 2, 2.5, 3},
			err:    nil, ir: 2, or: 4, filter: LinearFilter[float64]()},
		{name: "KaiserFast downsampling",
			input:  sine8000,
			output: sine125,
			err:    nil, ir: 8000, or: 125, filter: KaiserFastFilter[float64]()},
		{name: "KaiserFast uplampling",
			input:  sine125,
			output: sine8000,
			err:    nil, ir: 125, or: 8000, filter: KaiserFastFilter[float64]()},
		{name: "KaiserBest uplampling",
			input:  sine125,
			output: sine8000,
			err:    nil, ir: 125, or: 8000, filter: KaiserBestFilter[float64]()},
		{name: "Hanning uplampling",
			input:  sine125,
			output: sine8000,
			err:    nil, ir: 125, or: 8000, filter: HanningFilter[float64](64, 9)},
	}
	for _, tt := range resamplerTestFloat64 {
		t.Run(tt.name, func(t *testing.T) {
			outBuf := new(bytes.Buffer)
			inBuf := writeBuff(t, tt.input)

			res, err := New[float64](outBuf, tt.ir, tt.or, ch, tt.filter)
			assert.NoError(t, err)

			_, err = res.Write(inBuf.Bytes())
			if tt.err != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				output := readBuff[float64](t, outBuf, len(tt.output))
				if len(output) > 20 {
					assert.InDeltaSlicef(t, tt.output[10:10], output[10:10], .0001, "output: %v", output)
				} else {
					assert.InDeltaSlicef(t, tt.output, output, .0001, "output: %v", output)
				}
			}
		})
	}
}
func TestGetSincWindow(t *testing.T) {
	path := "./testdata/sinc_window_"
	zeros := 16
	density := 8

	file, err := os.OpenFile(path+"want", os.O_RDONLY, 0666)
	if errors.Is(err, os.ErrNotExist) {
		want := newHanningWindow(zeros, density)
		toFile(t, want, path+"got")
		t.Fatalf("Check saved results.\nRename file form *_got to *_want\nRun the test again")
	} else if err != nil {
		t.Fatal(err)
	}
	want := readBuff[float64](t, file, zeros*density+1)

	got := newHanningWindow(zeros, density)

	toFile(t, got, path+"got")
	assert.Lenf(t, got, len(want), "want: %d, got: %d", len(want), len(got))
	assert.InDeltaSlice(t, want, got, 0.0001)
}

func FuzzResampler(f *testing.F) {
	f.Fuzz(func(t *testing.T, samples []byte, ir, or int) {
		if len(samples)%2 != 0 {
			return
		}

		res, err := New[int16](io.Discard, ir, or, 1, LinearFilter[int16]())
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
