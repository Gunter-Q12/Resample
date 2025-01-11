package resample

import (
	"encoding/binary"
	"math"
	"os"
)

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

type KaiserFilter struct {
	interpWin   []float64
	interpDelta []float64
	density     int
}

func (k KaiserFilter) GetValue(position float64) float64 {
	sample := int(position)
	frac := position - float64(sample)

	return k.interpWin[sample] + frac*k.interpDelta[sample]
}

func (k KaiserFilter) GetDensity() int {
	return k.density
}

func (k KaiserFilter) GetLength() int {
	return len(k.interpWin)
}

func newKaiserFilter(path string, density, length int) KaiserFilter {
	interpWin := make([]float64, length)
	file, err := os.OpenFile(path, os.O_RDONLY, os.ModePerm)
	if err != nil {
		panic(err)
	}
	err = binary.Read(file, binary.LittleEndian, interpWin)
	if err != nil {
		panic(err)
	}

	interpDelta := getDifferences(interpWin)

	return KaiserFilter{
		interpWin:   interpWin,
		interpDelta: interpDelta,
		density:     density,
	}
}

func NewKaiserFastFilter() KaiserFilter {
	return newKaiserFilter("filters/kaiser_fast_f64", 512, 12289)
}

func NewKaiserBestFilter() KaiserFilter {
	return newKaiserFilter("filters/kaiser_best_f64", 8192, 409601)
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
