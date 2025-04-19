package resample_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/gunter-q12/resample"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/constraints"
	"io"
	"os"
	"reflect"
	"testing"
)

type number interface {
	constraints.Float | constraints.Integer
}

type testCase[T number] struct {
	name   string
	format resample.Format
	input  []T
	output []T
	err    error
	ir     int
	or     int
	ch     int
}

func TestResamplerInt(t *testing.T) {
	int16TestCases := []testCase[int16]{
		{name: "in=out", format: resample.FormatInt16,
			input: []int16{1, 2, 3}, output: []int16{1, 2, 3},
			err: nil, ir: 1, or: 1, ch: 1},
		{name: "simplest upsampling case", format: resample.FormatInt16,
			input: []int16{1, 3, 5}, output: []int16{1, 2, 3, 4, 5},
			err: nil, ir: 1, or: 2, ch: 1},
		{name: "simplest downsampling case", format: resample.FormatInt16,
			input: []int16{1, 2, 3, 4, 5}, output: []int16{1, 3},
			err: nil, ir: 2, or: 1, ch: 1},
		{name: "two channels", format: resample.FormatInt16,
			input:  []int16{1, 11, 3, 13, 5, 15},
			output: []int16{1, 11, 2, 12, 3, 13, 4, 14, 5, 15},
			err:    nil, ir: 1, or: 2, ch: 2},
	}

	for _, tc := range int16TestCases {
		runTestCase(t, "Memoization", tc, 0.001, resample.WithLinearFilter())
	}

	for _, tc := range int16TestCases {
		runTestCase(t, "No Memoization", tc, 0.001, resample.WithLinearFilter(), resample.WithNoMemoization())
	}

	t.Run("io.Copy", func(t *testing.T) {
		outBuf := new(bytes.Buffer)
		res, err := resample.New(outBuf, resample.FormatInt16, 1, 2, 1, resample.WithLinearFilter())
		require.NoError(t, err)

		inBuf := buffer(t, []int16{1, 3, 5})
		_, err = io.Copy(res, inBuf)

		require.NoError(t, err)
		output := unBuffer[int16](t, outBuf)
		assert.Equal(t, []int16{1, 2, 3, 4, 5}, output[:5])
	})
}

func TestResamplerFloat(t *testing.T) {
	linearTestCases := []testCase[float64]{
		{name: "simple downsampling", format: resample.FormatFloat64,
			input:  []float64{0, 0.25, 0.5, 0.75},
			output: []float64{0, 1.0 / 3, 2.0 / 3},
			err:    nil, ir: 4, or: 3, ch: 1},
		{name: "simple upsampling", format: resample.FormatFloat64,
			input:  []float64{1, 2, 3},
			output: []float64{1, 1.5, 2, 2.5, 3},
			err:    nil, ir: 2, or: 4, ch: 1},
	}

	for _, tc := range linearTestCases {
		runTestCase(t, "Memoization", tc, 0.001, resample.WithLinearFilter())
	}
	for _, tc := range linearTestCases {
		runTestCase(t, "No Memoization", tc, 0.001, resample.WithLinearFilter(), resample.WithNoMemoization())
	}
}

func TestResampleFloatKaiser(t *testing.T) {
	file, err := os.Open("./testdata/sine_8000_3_f64_ch1")
	require.NoError(t, err)
	sine8000 := unBuffer[float64](t, file)

	file, err = os.Open("./testdata/sine_125_3_f64_ch1")
	require.NoError(t, err)
	sine125 := unBuffer[float64](t, file)

	kaiserTestCases := []testCase[float64]{
		{name: "sine downsampling", format: resample.FormatFloat64,
			input:  sine8000,
			output: sine125,
			err:    nil, ir: 8000, or: 125, ch: 1},
		{name: "sine uplampling", format: resample.FormatFloat64,
			input:  sine125,
			output: sine8000,
			err:    nil, ir: 125, or: 8000, ch: 1},
	}
	filters := []struct {
		name string
		f    resample.Option
	}{
		{"fastest", resample.WithKaiserFastestFilter()},
		{"fast", resample.WithKaiserFastFilter()},
		{"best", resample.WithKaiserBestFilter()},
	}

	for _, tc := range kaiserTestCases {
		for _, f := range filters {
			runTestCase(
				t, "memoization "+f.name, tc, 0.01, f.f,
			)
		}
	}

	for _, tc := range kaiserTestCases {
		for _, f := range filters {
			runTestCase(
				t, "no memoization "+f.name, tc, 0.01, f.f,
			)
		}
	}
}

func runTestCase[T number](t *testing.T, nameSuffix string, tc testCase[T],
	delta float64, options ...resample.Option) {
	t.Run(fmt.Sprintf("%s %s", tc.name, nameSuffix), func(t *testing.T) {
		outBuf := new(bytes.Buffer)
		res, err := resample.New(outBuf, tc.format, tc.ir, tc.or, tc.ch, options...)
		require.NoError(t, err)

		_, err = res.Write(buffer(t, tc.input).Bytes())

		if tc.err != nil {
			assert.Error(t, err)
		} else {
			require.NoError(t, err)
			output := unBuffer[T](t, outBuf)
			commonLen := min(len(output), len(tc.output))
			assert.InDeltaSlicef(
				t, tc.output[:commonLen], output[:commonLen], delta,
				"output: %v", output,
			)
		}
	})
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

func buffer(t testing.TB, values any) *bytes.Buffer {
	inBuf := new(bytes.Buffer)
	err := binary.Write(inBuf, binary.LittleEndian, values)
	if err != nil {
		t.Fatal(err)
	}
	return inBuf
}

func unBuffer[T any](t testing.TB, buff io.Reader) []T {
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
