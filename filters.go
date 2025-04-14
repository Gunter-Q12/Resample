package resample

import (
	"embed"
	"encoding/binary"
	"fmt"
)

type Option func(*Resampler) error

func LinearFilter() Option {
	return func(r *Resampler) error {
		r.f = newFilter([]float64{1, 0}, 1, 1)
		return nil
	}
}

func KaiserFastestFilter() Option {
	return func(r *Resampler) error {
		interpWin, err := readWindowFromFile("filters/kaiser_super_fast_f64", 385)
		if err != nil {
			return fmt.Errorf("new kaiser best filter: %w", err)
		}

		scale := min(1.0, float64(r.outRate)/float64(r.inRate))
		r.f = newFilter(interpWin, 32, scale)

		return nil
	}
}

func KaiserFastFilter() Option {
	return func(r *Resampler) error {
		interpWin, err := readWindowFromFile("filters/kaiser_fast_f64", 12289)
		if err != nil {
			return fmt.Errorf("new kaiser best filter: %w", err)
		}

		scale := min(1.0, float64(r.outRate)/float64(r.inRate))
		r.f = newFilter(interpWin, 512, scale)

		return nil
	}
}

func KaiserBestFilter() Option {
	return func(r *Resampler) error {
		interpWin, err := readWindowFromFile("filters/kaiser_best_f64", 409601)
		if err != nil {
			return fmt.Errorf("new kaiser best filter: %w", err)
		}

		scale := min(1.0, float64(r.outRate)/float64(r.inRate))
		r.f = newFilter(interpWin, 8192, scale)

		return nil
	}
}

func HanningFilter(zeros, density int) Option {
	return func(r *Resampler) error {
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
	position := (offset + float64(index)) * k.scale * float64(k.density)
	integer := float64(int(position))
	frac := position - integer
	sampleId := int(integer)

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
