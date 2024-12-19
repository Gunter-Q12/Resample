package resample

import (
	"io"
)

type Quality int

const (
	Linear Quality = iota // Linear interpolation
)

type Format int

const (
	I16 Format = iota // 16-bit signed linear PCM
)

type Resampler struct {
	inRate      float64   // input sample rate
	outRate     float64   // output sample rate
	channels    int       // number of input channels
	format      Format    // input format
	quality     Quality   // resampling quality
	destination io.Writer // output destination
}

func New(destination io.Writer, ir, or float64, ch int, frmt Format, q Quality) (*Resampler, error) {
	return &Resampler{
		inRate:      ir,
		outRate:     or,
		channels:    ch,
		format:      frmt,
		quality:     q,
		destination: destination,
	}, nil
}
