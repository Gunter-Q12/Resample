package resample_test

import (
	"github.com/gunter-q12/resample"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestResamplerPrecision(t *testing.T) {
	music48Data, err := os.Open("./testdata/speech_sample_mono44.1kHz16bit.raw")
	require.NoError(t, err)
	music48 := unBuffer[int16](t, music48Data)

	music16Data, err := os.Open("./testdata/speech_sample_mono14.7kHz16bit.raw")
	require.NoError(t, err)
	music16 := unBuffer[int16](t, music16Data)

	testCases := []testCase[int16]{
		{name: "upsampling", format: resample.FormatInt16,
			input:  music16,
			output: music48,
			err:    nil, ir: 16000, or: 48000, ch: 1},
		{name: "downsampling", format: resample.FormatInt16,
			input:  music48,
			output: music16,
			err:    nil, ir: 48000, or: 16000, ch: 1},
	}

	filters := []struct {
		name string
		f    resample.Option
	}{
		{"fastest", resample.WithKaiserFastestFilter()},
		{"fast", resample.WithKaiserFastFilter()},
		{"best", resample.WithKaiserBestFilter()},
	}

	for _, tc := range testCases {
		for _, f := range filters {
			check(t, "memoization "+f.name, tc, avgDelta[int16](90), f.f)
		}
	}
	for _, tc := range testCases {
		for _, f := range filters {
			check(
				t, "no memoization "+f.name, tc, avgDelta[int16](90), f.f,
				resample.WithNoMemoization(),
			)
		}
	}
}

func avgDelta[T number](delta float64) checker[T] {
	return func(t *testing.T, expected, actual []T) {
		t.Helper()
		commonLen := min(len(expected), len(actual))

		var actualDelta float64
		for i := range expected[:commonLen] {
			delta := actual[i] - expected[i]
			if delta < 0 {
				delta = -delta
			}
			actualDelta += float64(delta)
		}
		actualDelta /= float64(commonLen)

		assert.LessOrEqual(t, actualDelta, delta)
	}
}
