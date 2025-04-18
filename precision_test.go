package resample_test

import (
	"bytes"
	"github.com/gunter-q12/resample"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"testing"
)

func TestResamplerPrecision(t *testing.T) {
	music48Data, err := os.Open("./testdata/speech_sample_mono44.1kHz16bit.raw")
	require.NoError(t, err)
	music48 := readBuff[int16](t, music48Data)

	music16Data, err := os.Open("./testdata/speech_sample_mono14.7kHz16bit.raw")
	require.NoError(t, err)
	music16 := readBuff[int16](t, music16Data)

	t.Run("Upsample", func(t *testing.T) {
		_, err = music16Data.Seek(0, io.SeekStart)
		require.NoError(t, err)

		out := new(bytes.Buffer)
		resampler, err := resample.New(out, resample.FormatInt16, 16000, 48000, 1)
		require.NoError(t, err)

		_, err = io.Copy(resampler, music16Data)
		require.NoError(t, err)
		music48Res := readBuff[int16](t, out)
		music48Res = music48Res[:len(music48Res)-2]

		t.Logf("original len: %d, resampled len: %d", len(music48), len(music48Res))
		stats(t, music48, music48Res)
	})

	t.Run("Downsample", func(t *testing.T) {
		_, err = music48Data.Seek(0, io.SeekStart)
		require.NoError(t, err)
		out := new(bytes.Buffer)
		resampler, err := resample.New(out, resample.FormatInt16, 48000, 16000, 1)
		require.NoError(t, err)

		_, err = io.Copy(resampler, music48Data)
		require.NoError(t, err)
		music16Res := readBuff[int16](t, out)

		t.Logf("original len: %d, resampled len: %d", len(music16[:len(music16Res)]), len(music16Res))
		stats(t, music16[:len(music16Res)], music16Res)
	})
}

func stats(t testing.TB, expected, actual []int16) {
	t.Helper()
	require.Len(t, expected, len(actual))

	var avgDelta int
	for i := range expected {
		delta := actual[i] - expected[i]
		if delta < 0 {
			delta = -delta
		}
		avgDelta += int(delta)
	}
	avgDelta /= len(expected)
	t.Logf("avg delta: %d", avgDelta)
	require.LessOrEqual(t, avgDelta, 90)
}
