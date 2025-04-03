package resample

import (
	"bytes"
	"encoding/binary"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"reflect"
	"testing"
)

func TestResamplerPrecision(t *testing.T) {
	music48Data, err := os.Open("./testdata/speech_sample_mono44.1kHz16bit.raw")
	require.NoError(t, err)
	music48 := readBuff2[int16](t, music48Data)

	music16Data, err := os.Open("./testdata/speech_sample_mono14.7kHz16bit.raw")
	require.NoError(t, err)

	out := new(bytes.Buffer)
	resampler, err := New(out, FormatInt16, 16000, 48000, 1)

	_, err = io.Copy(resampler, music16Data)
	music48Res := readBuff2[int16](t, out)
	music48Res = music48Res[:len(music48Res)-2]

	t.Logf("original len: %d, resampled len: %d", len(music48), len(music48Res))
	stats(t, music48, music48Res)
}

func stats(t testing.TB, expected, actual []int16) {
	t.Helper()
	require.Equal(t, len(expected), len(actual))

	var avgDelta int
	for i := range expected {
		delta := actual[i] - expected[i]
		if delta < 0 {
			delta = -delta
		}
		avgDelta += int(delta)
	}
	avgDelta = avgDelta / len(expected)
	t.Logf("avg delta: %d", avgDelta)
	require.LessOrEqual(t, avgDelta, 90)
}

func readBuff2[T any](t testing.TB, buff io.Reader) []T {
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
