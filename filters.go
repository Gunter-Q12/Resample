package resample

import (
	"embed"
	"encoding/binary"
	"fmt"
)

//go:embed filters
var filtersDir embed.FS

type filter struct {
	interpWin   []float64   // Window used for interpolation (scaled)
	interpDelta []float64   // Differences calculated as interpWin[i+1] - interpWin[i]
	offsetWins  [][]float64 // Window values at all points that may be used in calculations with current in/out ratio
	crossings   int         // Number of zero-crossings
	density     int         // Number of window values between two zero-crossings
	scale       float64     // Window scaling used during downsamplig to avoid aliasing
}

func newFilter(info filterInfo, inRate, outRate int, memoization bool) *filter {
	interpWin, err := readWindowFromFile(info.path, info.length)
	if err != nil {
		panic(fmt.Errorf("cannot open precompiled filter: %w", err))
	}

	scale := 1.0
	if info.isScaled {
		scale = min(1.0, float64(outRate)/float64(inRate))
	}

	n := len(interpWin)
	interpDelta := make([]float64, n)
	interpWin[0] *= scale
	for i := range n - 1 {
		interpWin[i+1] *= scale
		interpDelta[i] = interpWin[i+1] - interpWin[i]
	}

	f := &filter{
		interpWin:   interpWin,
		interpDelta: interpDelta,
		crossings:   info.length / info.density,
		density:     info.density,
		scale:       scale,
	}

	if !memoization {
		return f
	}

	// recalculating window values at all points that may be used
	var offsetWins [][]float64
	offsets := outRate / gcd(inRate, outRate)
	offsetWins = make([][]float64, offsets)
	timeIncrement := float64(inRate) / float64(outRate)
	for i := range offsets {
		offset := timeIncrement * float64(i)
		offset -= float64(int(offset))
		length := f.Length(offset)
		offsetWins[i] = make([]float64, length)
		for j := range length {
			offsetWins[i][j] = f.Value(offset, j)
		}
	}
	f.offsetWins = offsetWins
	f.interpDelta = nil
	f.interpDelta = nil

	return f
}

// Length is the number of samples that one wing of the window covers
// starting from given offset.
func (f filter) Length(offset float64) int {
	return int(float64(f.crossings)/f.scale - offset)
}

// Value is a window value at a given point.
//
// Point is provided as a fraction and integer parts.
func (f filter) Value(offset float64, index int) float64 {
	position := (offset + float64(index)) * f.scale * float64(f.density)
	integer := float64(int(position))
	frac := position - integer
	sampleID := int(integer)

	weight := f.interpWin[sampleID] + frac*f.interpDelta[sampleID]
	return weight
}

// readWindowFromFile reads precompiled filter window.
func readWindowFromFile(path string, length int) ([]float64, error) {
	op := "read window from file"

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

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
