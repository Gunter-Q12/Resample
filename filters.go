package resample

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

type Option[T Number] func(*Resampler[T]) error

func LinearFilter[T Number]() Option[T] {
	return func(r *Resampler[T]) error {
		r.filter = &linearFilter{}
		return nil
	}
}

func KaiserFastFilter[T Number]() Option[T] {
	return func(r *Resampler[T]) error {
		scale := min(1.0, float64(r.outRate)/float64(r.inRate))
		filter, err := newKaiserFilter("filters/kaiser_fast_f64", 512, 12289, scale)
		if err != nil {
			return fmt.Errorf("new kaiser fast filter: %w", err)
		}
		r.filter = filter
		return nil
	}
}

func KaiserBestFilter[T Number]() Option[T] {
	return func(r *Resampler[T]) error {
		scale := min(1.0, float64(r.outRate)/float64(r.inRate))
		filter, err := newKaiserFilter("filters/kaiser_best_f64", 8192, 409601, scale)
		if err != nil {
			return fmt.Errorf("new kaiser best filter: %w", err)
		}
		r.filter = filter
		return nil
	}
}

func HanningFilter[T Number](zeros, density int) Option[T] {
	return func(r *Resampler[T]) error {
		scale := min(1.0, float64(r.outRate)/float64(r.inRate))
		filter, err := newHanningFilter(zeros, density, scale)
		if err != nil {
			return fmt.Errorf("new kaiser best filter: %w", err)
		}
		r.filter = filter
		return nil
	}
}

type Filter interface {
	GetPoint(offset float64, index int) (float64, error)
}

type linearFilter struct {
}

func (lr linearFilter) GetPoint(offset float64, index int) (float64, error) {
	frac := offset + float64(index)
	return max(0, 1-frac), nil
}

type windowFilter struct {
	interpWin   []float64
	interpDelta []float64
	density     int
	scale       float64
}

func (k windowFilter) GetPoint(offset float64, index int) (float64, error) {
	integer, frac := math.Modf((offset + float64(index)) * k.scale)
	sampleId := int(integer * float64(k.density))

	if sampleId >= len(k.interpWin) {
		return 0, fmt.Errorf("sampleId out of range: %d", sampleId)
	}

	weight := k.interpWin[sampleId] + frac*k.interpDelta[sampleId]
	return weight, nil
}

func newKaiserFilter(path string, density, length int, scale float64) (*windowFilter, error) {
	interpWin := make([]float64, length)
	file, err := os.OpenFile(path, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("read pre-build kaiser filter: %w", err)
	}
	err = binary.Read(file, binary.LittleEndian, interpWin)
	if err != nil {
		return nil, fmt.Errorf("read pre-build kaiser filter: %w", err)
	}

	return newWindowFilter(interpWin, density, scale), nil
}

func newWindowFilter(interpWin []float64, density int, scale float64) *windowFilter {
	n := len(interpWin)
	interpDelta := make([]float64, n)

	for i := 0; i < n-1; i++ {
		interpDelta[i] = (interpWin[i+1] - interpWin[i]) * scale
		interpWin[i] *= scale
	}
	interpDelta[n-1] = 0
	interpWin[n-1] *= scale

	return &windowFilter{
		interpWin:   interpWin,
		interpDelta: interpDelta,
		density:     density,
	}
}

func newHanningFilter(zeros, density int, scale float64) (*windowFilter, error) {
	interpWin := newHanningWindow(zeros, density)
	return newWindowFilter(interpWin, density, scale), nil
}

func newHanningWindow(zeros, density int) []float64 {
	n := density * zeros
	sincWin := make([]float64, n+1)
	step := float64(zeros) / float64(n+1)
	for i := 0; i < n+1; i++ {
		sincWin[i] = sinc(float64(i) * step)
	}

	taper := hanning(2*n + 1)[n:]
	return elementwiseProd(taper, sincWin)
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
