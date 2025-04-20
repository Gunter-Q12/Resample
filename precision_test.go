package resample_test

import (
	"github.com/gunter-q12/resample"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

type filterCase struct {
	name string
	f    resample.Option
}

func TestResamplerPrecisionSpeech(t *testing.T) {
	speech44Data, err := os.Open("./testdata/speech_sample_mono44.1kHz16bit.raw")
	require.NoError(t, err)
	speech44 := unBuffer[int16](t, speech44Data)

	speech14Data, err := os.Open("./testdata/speech_sample_mono14.7kHz16bit.raw")
	require.NoError(t, err)
	speech14 := unBuffer[int16](t, speech14Data)

	downInput := testCase[int16]{
		name: "downsampling", format: resample.FormatInt16,
		input:  speech44,
		output: speech14,
		err:    nil, ir: 44100, or: 14700, ch: 1,
	}

	upInput := testCase[int16]{
		name: "upsampling", format: resample.FormatInt16,
		input:  speech14,
		output: speech44,
		err:    nil, ir: 14700, or: 44100, ch: 1,
	}

	// precision values were acquired from experiments with Resampy library
	// that implements the same interpolation method
	testCases := []struct {
		nameSuffix string
		tc         testCase[int16]
		filter     resample.Option
		precision  float64
	}{
		{
			nameSuffix: "fastest",
			tc:         upInput,
			filter:     resample.WithKaiserFastestFilter(),
			precision:  16,
		},
		{
			nameSuffix: "fast",
			tc:         upInput,
			filter:     resample.WithKaiserFastFilter(),
			precision:  23,
		},
		{
			nameSuffix: "best",
			tc:         upInput,
			filter:     resample.WithKaiserBestFilter(),
			precision:  14,
		},
		{
			nameSuffix: "fastest",
			tc:         downInput,
			filter:     resample.WithKaiserFastestFilter(),
			precision:  18,
		},
		{
			nameSuffix: "fast",
			tc:         downInput,
			filter:     resample.WithKaiserFastFilter(),
			precision:  24,
		},
		{
			nameSuffix: "best",
			tc:         downInput,
			filter:     resample.WithKaiserBestFilter(),
			precision:  16,
		},
	}

	for _, tc := range testCases {
		check(
			t, "memoization "+tc.nameSuffix,
			tc.tc, avgDelta[int16](tc.precision), tc.filter,
		)
	}
	for _, tc := range testCases {
		check(
			t, "no memoization "+tc.nameSuffix,
			tc.tc, avgDelta[int16](tc.precision), tc.filter,
		)
	}
}

// precision values were acquired from experiments with Resampy library
// that implements the same interpolation method
func TestResamplerPrecisionMusic(t *testing.T) {
	music48Data, err := os.Open("./testdata/music_sample_mono48kHz16bit.raw")
	require.NoError(t, err)
	music48 := unBuffer[int16](t, music48Data)

	music16Data, err := os.Open("./testdata/music_sample_mono16kHz16bit.raw")
	require.NoError(t, err)
	music16 := unBuffer[int16](t, music16Data)

	downInput := testCase[int16]{
		name: "downsampling", format: resample.FormatInt16,
		input:  music48,
		output: music16,
		err:    nil, ir: 48000, or: 16000, ch: 1,
	}

	upInput := testCase[int16]{
		name: "upsampling", format: resample.FormatInt16,
		input:  music16,
		output: music48,
		err:    nil, ir: 16000, or: 48000, ch: 1,
	}

	testCases := []struct {
		nameSuffix string
		tc         testCase[int16]
		filter     resample.Option
		precision  float64
	}{
		{
			nameSuffix: "fastest",
			tc:         upInput,
			filter:     resample.WithKaiserFastestFilter(),
			precision:  119,
		},
		{
			nameSuffix: "fast",
			tc:         upInput,
			filter:     resample.WithKaiserFastFilter(),
			precision:  173,
		},
		{
			nameSuffix: "best",
			tc:         upInput,
			filter:     resample.WithKaiserBestFilter(),
			precision:  97,
		},
		{
			nameSuffix: "fastest",
			tc:         downInput,
			filter:     resample.WithKaiserFastestFilter(),
			precision:  127,
		},
		{
			nameSuffix: "fast",
			tc:         downInput,
			filter:     resample.WithKaiserFastFilter(),
			precision:  179,
		},
		{
			nameSuffix: "best",
			tc:         downInput,
			filter:     resample.WithKaiserBestFilter(),
			precision:  107,
		},
	}

	for _, tc := range testCases {
		check(
			t, "memoization "+tc.nameSuffix,
			tc.tc, avgDelta[int16](tc.precision), tc.filter,
		)
	}
	for _, tc := range testCases {
		check(
			t, "no memoization "+tc.nameSuffix,
			tc.tc, avgDelta[int16](tc.precision), tc.filter,
		)
	}
}

func avgDelta[T number](delta float64) checker[T] {
	return func(t *testing.T, expected, actual []T) {
		t.Helper()
		commonLen := min(len(expected), len(actual))

		var actualDelta float64
		for i := range expected[:commonLen] {
			d := max(actual[i], expected[i]) - min(actual[i], expected[i])
			actualDelta += float64(d)
		}
		actualDelta /= float64(commonLen)

		t.Logf("average delta %.2f", actualDelta)
		assert.LessOrEqual(t, actualDelta, delta)
	}
}
