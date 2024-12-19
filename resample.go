package resample

type Quality int

const (
	Linear Quality = iota // Linear interpolation
)

func ResampleInt16(input []int16, ir, or, ch int, quality Quality) ([]int16, error) {
	return input, nil
}
