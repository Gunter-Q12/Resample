package resample

import (
	"embed"
	"encoding/binary"
	"fmt"
	"math"
)

type Option[T Number] func(*Resampler[T]) error

func LinearFilter[T Number]() Option[T] {
	return func(r *Resampler[T]) error {
		r.f = newFilter([]float64{1, 0}, 1, 1)
		return nil
	}
}

func KaiserFastFilter[T Number]() Option[T] {
	return func(r *Resampler[T]) error {
		interpWin, err := readWindowFromFile("filters/kaiser_fast_f64", 12289)
		if err != nil {
			return fmt.Errorf("new kaiser best filter: %w", err)
		}

		scale := min(1.0, float64(r.outRate)/float64(r.inRate))
		r.f = newFilter(interpWin, 512, scale)

		return nil
	}
}

func KaiserBestFilter[T Number]() Option[T] {
	return func(r *Resampler[T]) error {
		interpWin, err := readWindowFromFile("filters/kaiser_best_f64", 409601)
		if err != nil {
			return fmt.Errorf("new kaiser best filter: %w", err)
		}

		scale := min(1.0, float64(r.outRate)/float64(r.inRate))
		r.f = newFilter(interpWin, 8192, scale)

		return nil
	}
}

func HanningFilter[T Number](zeros, density int) Option[T] {
	return func(r *Resampler[T]) error {
		interpWin := newHanningWindow(zeros, density)

		scale := min(1.0, float64(r.outRate)/float64(r.inRate))
		r.f = newFilter(interpWin, density, scale)

		return nil
	}
}

type filter struct {
	interpWin   []float64
	interpDelta []float64
	density     int
	scale       float64
}

func (k filter) GetLength(offset float64) int {
	return int(
		(float64(len(k.interpWin)) - offset*k.scale*float64(k.density)) / k.scale / float64(k.density))
}

func (k filter) GetPoint(offset float64, index int) float64 {
	integer, frac := math.Modf((offset + float64(index)) * k.scale)
	sampleId := int(integer * float64(k.density))

	weight := k.interpWin[sampleId] + frac*k.interpDelta[sampleId]
	return weight
}

func newFilter(interpWin []float64, density int, scale float64) *filter {
	n := len(interpWin)
	interpDelta := make([]float64, n)

	for i := 0; i < n-1; i++ {
		interpDelta[i] = (interpWin[i+1] - interpWin[i]) * scale
		interpWin[i] *= scale
	}
	interpDelta[n-1] = 0
	interpWin[n-1] *= scale

	return &filter{
		interpWin:   interpWin,
		interpDelta: interpDelta,
		density:     density,
		scale:       scale,
	}
}

//go:embed filters
var filtersDir embed.FS

func readWindowFromFile(path string, length int) ([]float64, error) {
	op := "read filter from file"

	file, err := filtersDir.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	interpWin := make([]float64, length)
	err = binary.Read(file, binary.LittleEndian, interpWin)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return interpWin, nil
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
