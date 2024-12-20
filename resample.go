package resample

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

type Quality int

const (
	Linear Quality = iota // Linear interpolation
)

type Format int

const (
	I16 Format = iota
)

type Resampler struct {
	outBuf  io.Writer
	inRate  int
	outRate int
	ch      int
	format  Format
	quality Quality
}

func New(outBuffer io.Writer, inRate, outRate, ch int, format Format, quality Quality) *Resampler {
	return &Resampler{
		outBuf:  outBuffer,
		inRate:  inRate,
		outRate: outRate,
		ch:      ch,
		format:  format,
		quality: quality,
	}
}

func (r *Resampler) Write(input []byte) (int, error) {
	if r.inRate == r.outRate {
		return r.outBuf.Write(input)
	}

	if r.format != I16 {
		return 0, errors.New("the only supported format is I16")
	}

	samples := make([]int16, len(input)/2)
	err := binary.Read(bytes.NewReader(input), binary.LittleEndian, &samples)
	if err != nil {
		return 0, fmt.Errorf("resampler write: %w", err)
	}

	if len(samples) < 2 {
		return 0, errors.New("input should have at least two samples")
	}

	outputSize := int((len(samples)-1)*r.outRate/r.inRate) + 1
	output := make([]int16, outputSize)

	output[0] = samples[0]

	var x float64
	for i := 1; i < outputSize; i++ {
		x += float64(r.inRate) / float64(r.outRate)
		x0 := math.Floor(x)
		x1 := math.Ceil(x)

		x0Dist := x - x0
		x1Dist := x1 - x

		y0 := float64(samples[int(x0)])
		y1 := float64(samples[int(x1)])

		var newSample int16
		if math.Abs(x1-x0) < 1e-6 { // Why this epsilon?
			newSample = samples[int(x0)]
		} else {
			newSample = int16(y0*x1Dist + y1*x0Dist)
		}

		output[i] = newSample
	}

	err = binary.Write(r.outBuf, binary.LittleEndian, output)
	if err != nil {
		return 0, fmt.Errorf("resampler write: %w", err)
	}
	return len(input), nil
}
