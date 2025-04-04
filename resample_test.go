package resample

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"reflect"
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
		filter Option
	}{
		{name: "in=out",
			input: []int16{1, 2, 3}, output: []int16{1, 2, 3},
			err: nil, ir: 1, or: 1, ch: 1, filter: LinearFilter()},
		{name: "simplest upsampling case",
			input: []int16{1, 3, 5}, output: []int16{1, 2, 3, 4, 5},
			err: nil, ir: 1, or: 2, ch: 1, filter: LinearFilter()},
		{name: "simplest downsampling case",
			input: []int16{1, 2, 3, 4, 5}, output: []int16{1, 3},
			err: nil, ir: 2, or: 1, ch: 1, filter: LinearFilter()},
		{name: "two channels",
			input:  []int16{1, 11, 3, 13, 5, 15},
			output: []int16{1, 11, 2, 12, 3, 13, 4, 14, 5, 15},
			err:    nil, ir: 1, or: 2, ch: 2, filter: LinearFilter()},
	}
	for _, tt := range resamplerTestInt16 {
		t.Run(tt.name, func(t *testing.T) {
			outBuf := new(bytes.Buffer)
			inBuf := writeBuff(t, tt.input)

			res, err := New(outBuf, FormatInt16, tt.ir, tt.or, tt.ch, tt.filter)
			assert.NoError(t, err)

			_, err = res.Write(inBuf.Bytes())
			if tt.err != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				output := readBuff[int16](t, outBuf)
				assert.Equal(t, tt.output, output[:len(tt.output)])
			}
		})
	}

	t.Run("io.Copy", func(t *testing.T) {
		inBuf := writeBuff(t, []int16{1, 2, 3})
		outBuf := new(bytes.Buffer)

		res, err := New(outBuf, FormatInt16, 1, 2, 1, LinearFilter())
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
	sine8000 := readBuff[float64](t, file)

	file, err = os.Open("./testdata/sine_125_3_f64_ch1")
	if err != nil {
		t.Fatal(err)
	}
	sine125 := readBuff[float64](t, file)

	resamplerTestFloat64 := []struct {
		name   string
		input  []float64
		output []float64
		err    error
		ir     int
		or     int
		filter Option
	}{
		{name: "Linear downsampling",
			input:  []float64{0, 0.25, 0.5, 0.75},
			output: []float64{0, 1.0 / 3, 2.0 / 3},
			err:    nil, ir: 4, or: 3, filter: LinearFilter()},
		{name: "Linear upsampling",
			input:  []float64{1, 2, 3},
			output: []float64{1, 1.5, 2, 2.5, 3},
			err:    nil, ir: 2, or: 4, filter: LinearFilter()},
		{name: "KaiserFast downsampling",
			input:  sine8000,
			output: sine125,
			err:    nil, ir: 8000, or: 125, filter: KaiserFastFilter()},
		{name: "KaiserFast uplampling",
			input:  sine125,
			output: sine8000,
			err:    nil, ir: 125, or: 8000, filter: KaiserFastFilter()},
		{name: "KaiserBest uplampling",
			input:  sine125,
			output: sine8000,
			err:    nil, ir: 125, or: 8000, filter: KaiserBestFilter()},
		{name: "Hanning uplampling",
			input:  sine125,
			output: sine8000,
			err:    nil, ir: 125, or: 8000, filter: HanningFilter(64, 9)},
	}
	for _, tt := range resamplerTestFloat64 {
		t.Run(tt.name, func(t *testing.T) {
			outBuf := new(bytes.Buffer)
			inBuf := writeBuff(t, tt.input)

			res, err := New(outBuf, FormatFloat64, tt.ir, tt.or, ch, tt.filter)
			assert.NoError(t, err)

			_, err = res.Write(inBuf.Bytes())
			if tt.err != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				output := readBuff[float64](t, outBuf)
				if len(output) > 20 {
					assert.InDeltaSlicef(t, tt.output[10:10], output[10:10], .0001, "output: %v", output)
				} else {
					assert.InDeltaSlicef(t, tt.output, output[:len(tt.output)], .0001, "output: %v", output)
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
	want := readBuff[float64](t, file)

	got := newHanningWindow(zeros, density)

	toFile(t, got, path+"got")
	assert.Lenf(t, got, len(want), "want: %d, got: %d", len(want), len(got))
	assert.InDeltaSlice(t, want, got, 0.0001)
}

func FuzzResampler(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte, ir, or, ch int) {
		if ch <= 0 {
			ch = -ch + 1
		}
		samples := data[:len(data)/(2*ch)*(2*ch)]

		res, err := New(io.Discard, FormatInt16, ir, or, ch, LinearFilter())
		if err != nil {
			return
		}
		_, _ = res.Write(samples)
	})
}

func BenchmarkWrite(b *testing.B) {
	r, err := New(io.Discard, FormatFloat64, 8000, 44000, 2)
	assert.NoError(b, err)

	file, err := os.Open("./testdata/bench_samples.raw")
	if err != nil {
		b.Fatal(err)
	}
	samples, err := io.ReadAll(file)
	assert.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := r.Write(samples)
		assert.NoError(b, err)
	}
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

func writeBuff(t testing.TB, values any) *bytes.Buffer {
	inBuf := new(bytes.Buffer)
	err := binary.Write(inBuf, binary.LittleEndian, values)
	if err != nil {
		t.Fatal(err)
	}
	return inBuf
}

func readBuff[T any](t testing.TB, buff io.Reader) []T {
	t.Helper()
	var v T
	elemSize := int(reflect.TypeOf(v).Size())

	data, err := io.ReadAll(buff)
	require.NoError(t, err)

	output := make([]T, len(data)/elemSize)
	err = binary.Read(bytes.NewReader(data), binary.LittleEndian, output)
	require.NoError(t, err)

	return output
}
