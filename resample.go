package resample

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"golang.org/x/exp/constraints"
	"io"
	"math"
	"reflect"
)

type Number interface {
	constraints.Float | constraints.Integer
}

type Quality int

const (
	Linear Quality = iota // Linear interpolation
	KaiserFast
)

type Resampler[T Number] struct {
	outBuf  io.Writer
	inRate  int
	outRate int
	ch      int
	quality Quality
}

func New[T Number](outBuffer io.Writer, inRate, outRate, ch int, quality Quality) (*Resampler[T], error) {
	if inRate <= 0 || outRate <= 0 {
		return nil, errors.New("sampling rates must be greater than zero")
	}
	return &Resampler[T]{
		outBuf:  outBuffer,
		inRate:  inRate,
		outRate: outRate,
		ch:      ch,
		quality: quality,
	}, nil
}

func (r *Resampler[T]) Write(input []byte) (int, error) {
	if r.inRate == r.outRate {
		return r.outBuf.Write(input)
	}

	var t T
	elemLen := int(reflect.TypeOf(t).Size())

	samples := make([]T, len(input)/elemLen)
	err := binary.Read(bytes.NewReader(input), binary.LittleEndian, &samples)
	if err != nil {
		return 0, fmt.Errorf("resampler write: %w", err)
	}

	var output []T
	switch r.quality {
	case Linear:
		output, err = r.linear(samples)
	case KaiserFast:
		output, err = r.kaiserFast(samples)
	default:
		return 0, fmt.Errorf("unknown quality: %d", r.quality)
	}
	if err != nil {
		return 0, err
	}

	err = binary.Write(r.outBuf, binary.LittleEndian, output)
	if err != nil {
		return 0, fmt.Errorf("resampler write: %w", err)
	}
	return len(input), nil
}

func (r *Resampler[T]) linear(samples []T) ([]T, error) {
	if len(samples) < 2 {
		return nil, errors.New("input should have at least two samples")
	}

	outputSize := int((len(samples)-1)*r.outRate/r.inRate) + 1
	output := make([]T, outputSize)

	output[0] = samples[0]

	var x float64
	for i := 1; i < outputSize; i++ {
		x += float64(r.inRate) / float64(r.outRate)

		var newSample T
		if math.Abs(x-math.Round(x)) < 1e-6 { // Why this epsilon?
			newSample = samples[int(math.Round(x))]
		} else {
			x0 := math.Floor(x)
			x1 := math.Ceil(x)

			x0Dist := x - x0
			x1Dist := x1 - x

			y0 := float64(samples[int(x0)])
			y1 := float64(samples[int(x1)])

			newSample = T(y0*x1Dist + y1*x0Dist)
		}
		output[i] = newSample
	}

	return output, nil
}

func (r *Resampler[T]) kaiserFast(samples []T) ([]T, error) {
	return nil, errors.New("kaiser fast not implemented")
}

func getSincWindow(zeros, precision int, rolloff float64) ([]float64, error) {
	bits := 1 << precision
	n := bits * zeros
	sincWin := make([]float64, n+1)
	step := float64(zeros) / float64(n+1)
	for i := 0; i < n+1; i++ {
		sincWin[i] = rolloff * sinc(rolloff*(float64(i)*step))
	}

	taper := hanning(2*n + 1)[n:]

	interpWin := elementwiseProd(taper, sincWin)
	return interpWin, nil
}

func sinc(x float64) float64 {
	if x == 0 {
		return 1
	}
	return math.Sin(math.Pi*x) / (math.Pi * x)
}

func hanning(m int) []float64 {
	win := make([]float64, m)
	for n := 0; n < m; n++ {
		win[n] = 0.5 - 0.5*math.Cos((2*math.Pi*float64(n))/float64(m-1))
	}
	return win
}

func elementwiseProd(a, b []float64) []float64 {
	res := make([]float64, len(a))
	for i := 0; i < len(a); i++ {
		res[i] = a[i] * b[i]
	}
	return res
}
