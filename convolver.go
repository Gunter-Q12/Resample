package resample

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"runtime"
	"sync"
)

// convolver is a struct created before convolution and
// contains all the information necessary for it.
//
// The main purpose of this struct is to avoid memory allocations
// on each convolve call.
type convolver[T number] struct {
	r             *Resampler
	frameFunc     frameCalcFunc[T]
	timeIncrement float64
	processed     int

	convBuffer  []float64
	parseBuffer []T
	samples     []float64
	output      []T
}

// newConvolver returns a new convolver given a resampler and
// maximal possible input size in bytes.
func newConvolver[T number](r *Resampler, maxInputSize int) *convolver[T] {
	inFrames := maxInputSize / r.elemSize / r.ch
	outFrames := int(float64(inFrames*r.outRate) / float64(r.inRate))
	outSamples := outFrames * r.ch

	c := &convolver[T]{
		r:             r,
		timeIncrement: float64(r.inRate) / float64(r.outRate),

		convBuffer:  make([]float64, runtime.NumCPU()*routinesPerCore*r.ch),
		parseBuffer: make([]T, maxInputSize/r.elemSize),
		samples:     make([]float64, maxInputSize/r.elemSize),
		output:      make([]T, outSamples),
	}

	c.frameFunc = c.calcFrame
	if c.r.memoization {
		c.frameFunc = c.calcFrameWithMemoization
	}
	return c
}

// resample resamples part of a given input from a start to an end byte.
func (c *convolver[T]) resample(input []byte, start, end int) (int, error) {
	var err error
	err = c.parseSamples(input)
	if err != nil {
		return 0, fmt.Errorf("resampler: resample: %w", err)
	}

	startSample := start / c.r.elemSize
	endSample := end / c.r.elemSize

	inFrames := (endSample - startSample) / c.r.ch
	outFrames := int(float64(inFrames*c.r.outRate) / float64(c.r.inRate))
	outSamples := outFrames * c.r.ch
	c.output = c.output[:outSamples]

	c.convolve(startSample)

	err = binary.Write(c.r.outBuf, binary.LittleEndian, c.output[:outSamples])
	if err != nil {
		return 0, fmt.Errorf("resampler: resample: %w", err)
	}

	c.processed += outSamples
	return len(input), nil
}

// parseSamples parses input and loads it into samples field.
func (c *convolver[T]) parseSamples(input []byte) error {
	samples := c.parseBuffer[:len(input)/c.r.elemSize]
	err := binary.Read(bytes.NewReader(input), binary.LittleEndian, &samples)
	if err != nil {
		return fmt.Errorf("getting samples: %w", err)
	}

	c.samples = c.samples[:len(samples)]
	for i, s := range samples {
		c.samples[i] = float64(s)
	}
	return nil
}

// convolve performs convolution between samples and a filter window.
func (c *convolver[T]) convolve(startSample int) {
	ch := c.r.ch
	routines := runtime.NumCPU() * routinesPerCore
	frames := len(c.output) / ch
	framesPerRoutine := (frames + routines - 1) / routines
	if frames < routines {
		routines = 1
		framesPerRoutine = frames
	}

	wg := sync.WaitGroup{}
	for i := range routines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			startFrame := framesPerRoutine * i
			batchSize := min(framesPerRoutine, frames-startFrame)
			newSamples := c.convBuffer[i*ch : (i+1)*ch]
			for currFrame := range batchSize {
				outputFrame := startFrame + currFrame
				inputTime := float64(outputFrame) * c.timeIncrement
				inputFrame := int(inputTime) + (startSample / ch)

				c.frameFunc(newSamples, inputTime, inputFrame, outputFrame)

				start := outputFrame * ch
				for s, sample := range newSamples {
					c.output[start+s] = T(sample)
					newSamples[s] = 0
				}
			}
		}()
	}
	wg.Wait()
}

type frameCalcFunc[T number] func([]float64, float64, int, int)

// calcFrame calculates a single output frame and writes it to
// newSamples. Does not use precomputed window offsets.
func (c *convolver[T]) calcFrame(
	newSamples []float64, inputTime float64, inputFrame, _ int,
) {
	f := c.r.f
	ch := c.r.ch
	batchNum := len(c.samples) / ch

	offset := inputTime + float64(c.processed/ch)*c.timeIncrement
	offset -= float64(int(offset))

	// computing left wing including the middle element
	iters := min(f.Length(offset), inputFrame+1)
	for i := range iters {
		weight := f.Value(offset, i)
		startSample := (inputFrame - i) * ch
		for s := range newSamples {
			newSamples[s] += weight * c.samples[startSample+s]
		}
	}

	offset = 1 - offset

	// computing right wing
	iters = min(f.Length(offset), batchNum-1-inputFrame)
	for i := range iters {
		weight := f.Value(offset, i)
		startSample := (inputFrame + i + 1) * ch
		for s := range newSamples {
			newSamples[s] += weight * c.samples[startSample+s]
		}
	}
}

// calcFrameWithMemoization calculates a single output frame
// and writes it to newSamples. Uses precomputed window offsets.
func (c *convolver[T]) calcFrameWithMemoization(
	newSamples []float64, _ float64, inputFrame, outputFrame int,
) {
	f := c.r.f
	ch := c.r.ch
	batchNum := len(c.samples) / ch

	offsetsNum := len(f.offsetWins)
	offset := (outputFrame + c.processed) % offsetsNum

	// computing left wing including the middle element
	iters := min(len(f.offsetWins[offset]), inputFrame+1)
	for i, weight := range f.offsetWins[offset][:iters] {
		startSample := (inputFrame - i) * ch
		for s := range newSamples {
			newSamples[s] += weight * c.samples[startSample+s]
		}
	}

	offset = (offsetsNum - offset) % offsetsNum

	// computing right wing
	start := 0
	if offset == 0 { // avoid counting the first element twice
		start = 1
	}
	iters = min(len(f.offsetWins[offset]), batchNum-1-inputFrame)
	iters = max(start, iters)
	for i, weight := range f.offsetWins[offset][start:iters] {
		startSample := (inputFrame + i + 1) * ch
		for s := range newSamples {
			newSamples[s] += weight * c.samples[startSample+s]
		}
	}
}
