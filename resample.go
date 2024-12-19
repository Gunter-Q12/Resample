package resample

import (
	"errors"
	"math"
)

type Quality int

const (
	Linear Quality = iota // Linear interpolation
)

func Int16(input []int16, inRate, outRate, ch int, quality Quality) ([]int16, error) {
	if inRate == outRate {
		return input, nil
	}
	if len(input) < 2 {
		return []int16{}, errors.New("input should have at least two samples")
	}

	outputSize := int((len(input)-1)*outRate/inRate) + 1
	output := make([]int16, outputSize)
	output[0] = input[0]

	var x float64
	for i := 1; i < outputSize; i++ {
		x += float64(inRate) / float64(outRate)
		x0 := math.Floor(x)
		x1 := math.Ceil(x)

		x0Dist := x - x0
		x1Dist := x1 - x

		y0 := float64(input[int(x0)])
		y1 := float64(input[int(x1)])

		var newSample int16
		if math.Abs(x1-x0) < 1e-6 { // Why this epsilon?
			newSample = input[int(x0)]
		} else {
			newSample = int16(y0*x1Dist + y1*x0Dist)
		}

		output[i] = newSample
	}
	return output, nil
}
