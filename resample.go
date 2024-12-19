package resample

import "io"

const (
	Linear    = -1 // Linear interpolation
	Quick     = 0  // Quick cubic interpolation
	LowQ      = 1  // LowQ 16-bit with larger rolloff
	MediumQ   = 2  // MediumQ 16-bit with medium rolloff
	HighQ     = 4  // High quality
	VeryHighQ = 6  // Very high quality

	F32 = 0 // 32-bit floating point PCM
	F64 = 1 // 64-bit floating point PCM
	I32 = 2 // 32-bit signed linear PCM
	I16 = 3 // 16-bit signed linear PCM

	byteLen = 8
)

type Resampler struct {
	inRate      float64   // input sample rate
	outRate     float64   // output sample rate
	channels    int       // number of input channels
	inFormat    int       // input format
	destination io.Writer // output data
}

func New(destination io.Writer, ir, or float64, ch, frmt, quality int) (*Resampler, error) {
	return &Resampler{
		inRate:      ir,
		outRate:     or,
		channels:    ch,
		inFormat:    frmt,
		destination: destination,
	}, nil
}
