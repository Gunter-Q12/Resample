package resample

import (
	"embed"
	"encoding/binary"
	"fmt"
)

//go:embed filters
var filtersDir embed.FS

type filter struct {
	precalcWins [][]float64
	interpWin   []float64
	interpDelta []float64
	density     int
	scale       float64
}

func newFilter(info filterInfo, inRate, outRate, memLimit int) *filter {
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

	for i := range n - 1 {
		interpDelta[i] = (interpWin[i+1] - interpWin[i]) * scale
		interpWin[i] *= scale
	}
	interpDelta[n-1] = 0
	interpWin[n-1] *= scale

	f := &filter{
		interpWin:   interpWin,
		interpDelta: interpDelta,
		density:     info.density,
		scale:       scale,
	}

	// recalculating all the offsets
	var precalcWins [][]float64
	copies := outRate / gcd(inRate, outRate)
	if memLimit >= 0 && copies*len(interpWin)*8 > memLimit {
		return f
	}
	precalcWins = make([][]float64, copies)
	timeIncrement := float64(inRate) / float64(outRate)
	for i := range copies {
		offset := timeIncrement * float64(i)
		offset -= float64(int(offset))
		length := f.GetLength(offset)
		precalcWins[i] = make([]float64, length)
		for j := range length {
			precalcWins[i][j] = f.GetPoint(offset, j)
		}
	}
	f.interpDelta = nil
	f.interpWin = nil
	f.precalcWins = precalcWins
	return f
}

func (k filter) GetLength(offset float64) int {
	return int(
		(float64(len(k.interpWin)) - offset*k.scale*float64(k.density)) / k.scale / float64(k.density))
}

func (k filter) GetPoint(offset float64, index int) float64 {
	position := (offset + float64(index)) * k.scale * float64(k.density)
	integer := float64(int(position))
	frac := position - integer
	sampleID := int(integer)

	weight := k.interpWin[sampleID] + frac*k.interpDelta[sampleID]
	return weight
}

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

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
