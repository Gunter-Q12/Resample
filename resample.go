package resample

import (
	"errors"
	"golang.org/x/exp/constraints"
	"io"
	"runtime"
	"slices"
)

const routinesPerCore = 4

type number interface {
	constraints.Float | constraints.Integer
}

type Format int

const (
	FormatInt16 Format = iota
	FormatInt32
	FormatInt64
	FormatFloat32
	FormatFloat64
)

//nolint:mnd // map used as a constant
var formatElementSize = map[Format]int{
	FormatInt16:   2,
	FormatInt32:   4,
	FormatInt64:   8,
	FormatFloat32: 4,
	FormatFloat64: 8,
}

// A Resampler is a struct used for resampling.
type Resampler struct {
	outBuf      io.Writer
	format      Format
	inRate      int
	outRate     int
	ch          int
	memoization bool
	f           *filter
	elemSize    int
}

// New creates a new Resampler.
//
// Calling Resampler.Write and Resampler.ReadFrom methods on the returned Resampler
// will resample data according to provided format, inRate, outRate and number of channels.
// Results are written to the io.Writer.
//
// Default filter is KaiserFastFilter, use WithXFilter options to change it.
// Memoization is enabled by default, use WithNoMemoization function to disable it.
func New(outBuffer io.Writer, format Format, inRate, outRate, ch int,
	options ...Option) (*Resampler, error) {
	if inRate <= 0 || outRate <= 0 || ch <= 0 {
		return nil, errors.New("sampling rates and channel number must be greater than zero")
	}

	resampler := &Resampler{
		outBuf:      outBuffer,
		format:      format,
		inRate:      inRate,
		outRate:     outRate,
		ch:          ch,
		memoization: true,
		elemSize:    formatElementSize[format],
	}

	slices.SortFunc(options, optionCmp)
	for _, option := range options {
		if err := option.apply(resampler); err != nil {
			return nil, err
		}
	}

	if resampler.f == nil {
		if err := WithKaiserFastFilter().apply(resampler); err != nil {
			return nil, err
		}
	}

	return resampler, nil
}

// Write writes resampled data to an io.Writer provided during a New call.
//
// If Write is called multiple times, the data is appended to the same io.Writer.
// Note that calling Write on separate parts of the same file may result in
// imperfect resampling at the boundaries. For large files that do not fit
// into memory, use io.Copy instead.
func (r *Resampler) Write(input []byte) (int, error) {
	switch r.format {
	case FormatInt16:
		return write[int16](r, input)
	case FormatInt32:
		return write[int32](r, input)
	case FormatInt64:
		return write[int64](r, input)
	case FormatFloat32:
		return write[float32](r, input)
	case FormatFloat64:
		return write[float64](r, input)
	default:
		panic("unknown format")
	}
}

// ReadFrom reads all the data from reader using batching to reduce memory usage.
func (r *Resampler) ReadFrom(reader io.Reader) (int64, error) {
	switch r.format {
	case FormatInt16:
		return readFrom[int16](r, reader)
	case FormatInt32:
		return readFrom[int32](r, reader)
	case FormatInt64:
		return readFrom[int64](r, reader)
	case FormatFloat32:
		return readFrom[float32](r, reader)
	case FormatFloat64:
		return readFrom[float64](r, reader)
	default:
		panic("unknown format")
	}
}

// write is an actual implementation of Resampler.Write
func write[T number](r *Resampler, input []byte) (int, error) {
	c := newConvolver[T](r, len(input))
	return c.resample(input, 0, len(input))
}

// readFrom is an actual implementation of Resampler.ReadFrom
func readFrom[T number](r *Resampler, reader io.Reader) (int64, error) {
	wingSize := r.f.Length(0) * r.elemSize
	middleSize := (runtime.NumCPU()*1024 + r.inRate - 1) / r.inRate * r.inRate
	buffSize := wingSize*3 + middleSize*r.elemSize //nolint:mnd // math

	buff := make([]byte, buffSize)
	c := newConvolver[T](r, buffSize)
	read := 0

	n, err := reader.Read(buff[:middleSize+wingSize])
	read += n
	if err != nil && errors.Is(err, io.EOF) {
		return int64(read), err
	}
	if n < middleSize+wingSize {
		_, err = c.resample(buff[:n], 0, n)
		return int64(read), err
	}

	// Special case for the first part
	_, err = c.resample(buff[:middleSize], 0, middleSize)
	if err != nil {
		return int64(read), err
	}
	_ = copy(buff[:wingSize*2], buff[middleSize-wingSize:middleSize+wingSize])

	for {
		n, err = reader.Read(buff[wingSize*2 : wingSize*2+middleSize])
		read += n
		if err != nil && errors.Is(err, io.EOF) {
			return int64(read), err
		}
		if n < middleSize {
			_, err = c.resample(buff[0:wingSize*2+n], wingSize, wingSize*2+n)
			return int64(read), err
		}

		_, err = c.resample(buff[0:wingSize*2+middleSize], wingSize, wingSize+middleSize)
		if err != nil {
			return int64(read), err
		}
		_ = copy(buff[:wingSize*2], buff[middleSize:middleSize+wingSize*2])
	}
}
