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
	outBuf   io.Writer
	inRate   int
	outRate  int
	ch       int
	quality  Quality
	elemSize int
}

func New[T Number](outBuffer io.Writer, inRate, outRate, ch int, quality Quality) (*Resampler[T], error) {
	if inRate <= 0 || outRate <= 0 {
		return nil, errors.New("sampling rates must be greater than zero")
	}

	var t T
	elemSize := int(reflect.TypeOf(t).Size())

	return &Resampler[T]{
		outBuf:   outBuffer,
		inRate:   inRate,
		outRate:  outRate,
		ch:       ch,
		quality:  quality,
		elemSize: elemSize,
	}, nil
}

func (r *Resampler[T]) Write(input []byte) (int, error) {
	if r.inRate == r.outRate {
		return r.outBuf.Write(input)
	}

	samples := make([]T, len(input)/r.elemSize)
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
		err = fmt.Errorf("unknown quality: %d", r.quality)
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

	ratio := float64(r.outRate) / float64(r.inRate)
	shape := int(float64(len(samples)) * float64(r.outRate) / float64(r.inRate))

	scale := 1.0
	timeIncrement := 1.0 / ratio

	timeOut := make([]float64, shape)
	for i := range timeOut {
		timeOut[i] = float64(i) * timeIncrement
	}

	interpWin := []float64{1, 0}
	precision := 1

	interpDelta := getDifferences(interpWin)

	y := make([]T, shape)
	r.resample(samples, timeOut, interpWin, interpDelta, precision, scale, &y)
	return y, nil
}

func (r *Resampler[T]) kaiserFast(samples []T) ([]T, error) {
	return nil, errors.New("kaiser fast not implemented")
}

func (r *Resampler[T]) resample(samples []T, timeOut, interpWin, interpDelta []float64,
	numTable int, scale float64, y *[]T) {

	indexStep := int(scale * float64(numTable))
	timeRegister := 0.0

	nWin := len(interpWin)
	nIn := len(samples)

	for t := range timeOut {
		var newSample float64
		timeRegister = timeOut[t]

		n := int(timeRegister)

		frac := scale * (timeRegister - float64(n))

		indexFrac := frac * float64(numTable)
		offset := int(indexFrac)

		eta := indexFrac - float64(offset)

		iMax := min(n+1, (nWin-offset)/indexStep)
		for i := 0; i < iMax; i++ {
			weight := interpWin[offset+i*indexStep] +
				eta*interpDelta[offset+i*indexStep]
			newSample += weight * float64(samples[n-i])
		}

		frac = scale - frac

		indexFrac = frac * float64(numTable)
		offset = int(indexFrac)

		eta = indexFrac - float64(offset)

		kMax := min(nIn-n-1, (nWin-offset)/indexStep)
		for k := 0; k < kMax; k++ {
			weight := interpWin[offset+k*indexStep] +
				eta*interpDelta[offset+k*indexStep]
			newSample += weight * float64(samples[n+k+1])
		}
		(*y)[t] = T(newSample) // TODO: rounding
	}
}

func getSincWindow(zeros, precision int) ([]float64, error) {
	n := precision * zeros
	sincWin := make([]float64, n+1)
	step := float64(zeros) / float64(n+1)
	for i := 0; i < n+1; i++ {
		sincWin[i] = sinc(float64(i) * step)
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

func getDifferences(values []float64) []float64 {
	diffs := make([]float64, len(values))
	for i := 0; i < len(values)-1; i++ {
		diffs[i] = values[i+1] - values[i]
	}
	diffs[len(diffs)-1] = 0
	return diffs
}
