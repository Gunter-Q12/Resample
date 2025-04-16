package resample_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gunter-go/resample"
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
		filter resample.Option
	}{
		{name: "in=out",
			input: []int16{1, 2, 3}, output: []int16{1, 2, 3},
			err: nil, ir: 1, or: 1, ch: 1, filter: resample.WithLinearFilter()},
		{name: "simplest upsampling case",
			input: []int16{1, 3, 5}, output: []int16{1, 2, 3, 4, 5},
			err: nil, ir: 1, or: 2, ch: 1, filter: resample.WithLinearFilter()},
		{name: "simplest downsampling case",
			input: []int16{1, 2, 3, 4, 5}, output: []int16{1, 3},
			err: nil, ir: 2, or: 1, ch: 1, filter: resample.WithLinearFilter()},
		{name: "two channels",
			input:  []int16{1, 11, 3, 13, 5, 15},
			output: []int16{1, 11, 2, 12, 3, 13, 4, 14, 5, 15},
			err:    nil, ir: 1, or: 2, ch: 2, filter: resample.WithLinearFilter()},
	}
	for _, tt := range resamplerTestInt16 {
		t.Run(tt.name, func(t *testing.T) {
			outBuf := new(bytes.Buffer)
			inBuf := writeBuff(t, tt.input)

			res, err := resample.New(outBuf, resample.FormatInt16, tt.ir, tt.or, tt.ch, tt.filter)
			require.NoError(t, err)

			_, err = res.Write(inBuf.Bytes())
			if tt.err != nil {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				output := readBuff[int16](t, outBuf)
				assert.Equal(t, tt.output, output[:len(tt.output)])
			}
		})
	}

	t.Run("io.Copy", func(t *testing.T) {
		inBuf := writeBuff(t, []int16{1, 2, 3})
		outBuf := new(bytes.Buffer)

		res, err := resample.New(outBuf, resample.FormatInt16, 1, 2, 1, resample.WithLinearFilter())
		require.NoError(t, err)

		size, err := io.Copy(res, inBuf)
		require.NoError(t, err)
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
		filter resample.Option
	}{
		{name: "Linear downsampling",
			input:  []float64{0, 0.25, 0.5, 0.75},
			output: []float64{0, 1.0 / 3, 2.0 / 3},
			err:    nil, ir: 4, or: 3, filter: resample.WithLinearFilter()},
		{name: "Linear upsampling",
			input:  []float64{1, 2, 3},
			output: []float64{1, 1.5, 2, 2.5, 3},
			err:    nil, ir: 2, or: 4, filter: resample.WithLinearFilter()},
		{name: "KaiserFast downsampling",
			input:  sine8000,
			output: sine125,
			err:    nil, ir: 8000, or: 125, filter: resample.WithKaiserFastFilter()},
		{name: "KaiserFast uplampling",
			input:  sine125,
			output: sine8000,
			err:    nil, ir: 125, or: 8000, filter: resample.WithKaiserFastFilter()},
		{name: "KaiserBest uplampling",
			input:  sine125,
			output: sine8000,
			err:    nil, ir: 125, or: 8000, filter: resample.WithKaiserBestFilter()},
	}
	for _, tt := range resamplerTestFloat64 {
		t.Run(tt.name, func(t *testing.T) {
			outBuf := new(bytes.Buffer)
			inBuf := writeBuff(t, tt.input)

			res, err := resample.New(outBuf, resample.FormatFloat64, tt.ir, tt.or, ch, tt.filter)
			require.NoError(t, err)

			_, err = res.Write(inBuf.Bytes())
			if tt.err != nil {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
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

func FuzzResampler(f *testing.F) {
	f.Fuzz(func(_ *testing.T, data []byte, ir, or, ch int) {
		if ch <= 0 {
			ch = -ch + 1
		}
		samples := data[:len(data)/(2*ch)*(2*ch)]

		res, err := resample.New(io.Discard, resample.FormatInt16, ir, or, ch, resample.WithLinearFilter())
		if err != nil {
			return
		}
		_, _ = res.Write(samples)
	})
}

func BenchmarkWrite(b *testing.B) {
	r, err := resample.New(io.Discard, resample.FormatFloat64, 8000, 44000, 2)
	require.NoError(b, err)

	file, err := os.Open("./testdata/bench_samples.raw")
	if err != nil {
		b.Fatal(err)
	}
	samples, err := io.ReadAll(file)
	require.NoError(b, err)

	b.ResetTimer()
	for range b.N {
		_, err := r.Write(samples)
		require.NoError(b, err)
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

func Example_resamplingFile() {
	input, _ := os.Open("./original.raw")
	output, _ := os.Create("./resampled.raw")

	res, _ := resample.New(output, resample.FormatInt16, 48000, 16000, 2)
	_, _ = io.Copy(res, input)
}

func Example_resamplingSlice() {
	// Convert slice of values into a slice of bytes
	input := []int16{1, 3, 5}
	inputData := new(bytes.Buffer)
	_ = binary.Write(inputData, binary.LittleEndian, input)

	// Resample
	outBuf := new(bytes.Buffer)
	res, _ := resample.New(outBuf, resample.FormatInt16, 1, 2, 1, resample.WithLinearFilter())
	_, _ = res.Write(inputData.Bytes())

	// Convert bytes back to a slice of values
	output := make([]int16, 5)
	_ = binary.Read(outBuf, binary.LittleEndian, output)

	fmt.Println(output)

	// Output: [1 2 3 4 5]
}
