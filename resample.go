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

type Filter interface {
	GetValue(position float64) float64
	GetDensity() int
	GetLength() int
}

type LinearFilter struct {
}

func (lr LinearFilter) GetValue(position float64) float64 {
	return max(0, 1-position)
}

func (lr LinearFilter) GetDensity() int {
	return 1
}

func (lr LinearFilter) GetLength() int {
	return 2
}

func NewLinearFilter() LinearFilter {
	return LinearFilter{}
}

type KaiserFastFilter struct {
	interpWin   []float64
	interpDelta []float64
	density     int
}

func (k KaiserFastFilter) GetValue(position float64) float64 {
	sample := int(position)
	frac := position - float64(sample)

	return k.interpWin[sample] + frac*k.interpDelta[sample]
}

func (k KaiserFastFilter) GetDensity() int {
	return k.density
}

func (k KaiserFastFilter) GetLength() int {
	return len(k.interpWin)
}

func NewKaiserFastFilter() KaiserFastFilter {
	density := 512
	interpWin, _ := getSincWindow(24, density)
	interpDelta := getDifferences(interpWin)
	return KaiserFastFilter{
		interpWin:   interpWin,
		interpDelta: interpDelta,
		density:     density,
	}
}

type Resampler[T Number] struct {
	outBuf   io.Writer
	inRate   int
	outRate  int
	ch       int
	filter   Filter
	elemSize int
}

func New[T Number](outBuffer io.Writer, inRate, outRate, ch int, filter Filter) (*Resampler[T], error) {
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
		filter:   filter,
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
	switch t := r.filter.(type) {
	case LinearFilter:
		output, err = r.linear(samples)
	case KaiserFastFilter:
		output, err = r.kaiserFast(samples)
	default:
		err = fmt.Errorf("custom filters are not supported: %v", t)
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

	y := make([]T, shape)
	r.resample(samples, timeOut, scale, &y)
	return y, nil
}

func (r *Resampler[T]) kaiserFast(samples []T) ([]T, error) {
	ratio := float64(r.outRate) / float64(r.inRate)
	shape := int(float64(len(samples)) * float64(r.outRate) / float64(r.inRate))

	timeIncrement := 1.0 / ratio
	timeOut := make([]float64, shape)
	for i := range timeOut {
		timeOut[i] = float64(i) * timeIncrement
	}

	scale := min(1.0, float64(r.outRate)/float64(r.inRate))

	y := make([]T, shape)
	r.resample(samples, timeOut, scale, &y)
	return y, nil
}

func (r *Resampler[T]) resample(samples []T, timeOut []float64, scale float64, y *[]T) {

	winLen := r.filter.GetLength()
	samplesLen := len(samples)

	for t := range *y {
		var newSample float64

		timeRegister := timeOut[t]

		sampleId := int(timeRegister)
		frac := scale * (timeRegister - float64(sampleId))
		step := float64(r.filter.GetDensity()) * scale
		filterId := int(frac * float64(r.filter.GetDensity()))
		frac -= float64(filterId) * (1 / float64(r.filter.GetDensity()))

		// computing left wing (because of the middle element)
		i := 0
		for sampleId-i >= 0 && int(float64(filterId)+step*float64(i)) < winLen {
			currFilter := int(float64(filterId) + step*float64(i))
			currFrac := frac + step*float64(i) - float64(int(frac+step*float64(i)))

			weight := scale * r.filter.GetValue(float64(currFilter)+currFrac)
			newSample += weight * float64(samples[sampleId-i])
			i += 1
		}

		frac = scale * (1 - (timeRegister - float64(sampleId)))
		filterId = int(frac * float64(r.filter.GetDensity()))
		frac -= float64(filterId) * (1 / float64(r.filter.GetDensity()))

		// computing right wing
		i = 0
		for (sampleId+i+1) < samplesLen && int(float64(filterId)+step*float64(i)) < winLen {
			currFilter := int(float64(filterId) + step*float64(i))
			currFrac := frac + step*float64(i) - float64(int(frac+step*float64(i)))

			weight := scale * r.filter.GetValue(float64(currFilter)+currFrac)
			newSample += weight * float64(samples[sampleId+i+1])
			i += 1
		}
		(*y)[t] = T(newSample) // TODO: proper rounding
	}
}

func getSincWindow(zeros, density int) ([]float64, error) {
	n := density * zeros
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
