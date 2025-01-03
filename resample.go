package resample

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

type Quality int

const (
	Linear Quality = iota // Linear interpolation
	KaiserFast
)

type Format int

const (
	I16 Format = iota
)

type Resampler struct {
	outBuf  io.Writer
	inRate  int
	outRate int
	ch      int
	format  Format
	quality Quality
}

func New(outBuffer io.Writer, inRate, outRate, ch int, format Format, quality Quality) (*Resampler, error) {
	if inRate <= 0 || outRate <= 0 {
		return nil, errors.New("sampling rates must be greater than zero")
	}
	return &Resampler{
		outBuf:  outBuffer,
		inRate:  inRate,
		outRate: outRate,
		ch:      ch,
		format:  format,
		quality: quality,
	}, nil
}

func (r *Resampler) Write(input []byte) (int, error) {
	if r.inRate == r.outRate {
		return r.outBuf.Write(input)
	}

	if r.format != I16 {
		return 0, errors.New("the only supported format is I16")
	}

	samples := make([]int16, len(input)/2)
	err := binary.Read(bytes.NewReader(input), binary.LittleEndian, &samples)
	if err != nil {
		return 0, fmt.Errorf("resampler write: %w", err)
	}

	switch r.quality {
	case Linear:
		return r.linear(samples)
	case KaiserFast:
		return r.kaiserFast(samples)
	default:
		return 0, fmt.Errorf("unknown quality: %d", r.quality)
	}
}

func (r *Resampler) linear(samples []int16) (int, error) {
	if len(samples) < 2 {
		return 0, errors.New("input should have at least two samples")
	}

	outputSize := int((len(samples)-1)*r.outRate/r.inRate) + 1
	output := make([]int16, outputSize)

	output[0] = samples[0]

	var x float64
	for i := 1; i < outputSize; i++ {
		x += float64(r.inRate) / float64(r.outRate)

		var newSample int16
		if math.Abs(x-math.Round(x)) < 1e-6 { // Why this epsilon?
			newSample = samples[int(math.Round(x))]
		} else {
			x0 := math.Floor(x)
			x1 := math.Ceil(x)

			x0Dist := x - x0
			x1Dist := x1 - x

			y0 := float64(samples[int(x0)])
			y1 := float64(samples[int(x1)])

			newSample = int16(y0*x1Dist + y1*x0Dist)
		}
		output[i] = newSample
	}

	err := binary.Write(r.outBuf, binary.LittleEndian, output)
	if err != nil {
		return 0, fmt.Errorf("resampler write: %w", err)
	}
	return len(samples) * 2, nil
}

func (r *Resampler) kaiserFast(samples []int16) (int, error) {
	return 0, errors.New("kaiser fast not implemented")
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
