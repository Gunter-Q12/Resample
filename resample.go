package resample

import "errors"

type Quality int

const (
	Linear Quality = iota // Linear interpolation
)

func ResampleInt16(input []int16, ir, or, ch int, quality Quality) ([]int16, error) {
	if ir == or {
		return input, nil
	}
	if len(input) < 2 {
		return []int16{}, errors.New("input should have at least two samples")
	}
	return []int16{}, nil
}
