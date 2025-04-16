package resample

const (
	filterPrecedence      = 50
	memoryLimitPrecedence = 100
)

// Option is a struct used to configure [Resampler].
type Option struct {
	precedence int
	apply      func(*Resampler) error
}

func optionCmp(a, b Option) int {
	return b.precedence - a.precedence
}

func WithMemoryLimit(bytes int) Option {
	return Option{
		precedence: memoryLimitPrecedence,
		apply: func(r *Resampler) error {
			r.memLimit = bytes
			return nil
		},
	}
}

// fileInfo stores info about precompiled filters
type filterInfo struct {
	path     string
	length   int
	density  int
	isScaled bool
}

//nolint:mnd // structs used as constants
var (
	linearInfo = filterInfo{
		path:     "filters/linear_f64",
		length:   2,
		density:  1,
		isScaled: false,
	}
	kaiserFastestInfo = filterInfo{
		path:     "filters/kaiser_fastest_f64",
		length:   385,
		density:  32,
		isScaled: true,
	}
	kaiserFastInfo = filterInfo{
		path:     "filters/kaiser_fast_f64",
		length:   12289,
		density:  512,
		isScaled: true,
	}
	kaiserBestInfo = filterInfo{
		path:     "filters/kaiser_best_f64",
		length:   409601,
		density:  8192,
		isScaled: true,
	}
)

// WithLinearFilter function returns option that configures [Resampler] to use linear filter.
//
// This option should be used only for testing purposes because linear filter provides
// poor resampling quality.
func WithLinearFilter() Option {
	return Option{
		precedence: filterPrecedence,
		apply: func(r *Resampler) error {
			r.f = newFilter(linearInfo, r.inRate, r.outRate, r.memLimit)
			return nil
		},
	}
}

// WithKaiserFastestFilter function returns option
// that configures [Resampler] to use the fast Kaiser filter.
//
// Fastest Kaiser filter provides higher resampling speed in exchange for lower quality.
func WithKaiserFastestFilter() Option {
	return Option{
		precedence: filterPrecedence,
		apply: func(r *Resampler) error {
			r.f = newFilter(kaiserFastestInfo, r.inRate, r.outRate, r.memLimit)
			return nil
		},
	}
}

// WithKaiserFastFilter function returns option
// that configures [Resampler] to use the best Kaiser filter.
//
// Best Kaiser filter provides higher resampling speed in exchange for lower quality.
func WithKaiserFastFilter() Option {
	return Option{
		precedence: filterPrecedence,
		apply: func(r *Resampler) error {
			r.f = newFilter(kaiserFastInfo, r.inRate, r.outRate, r.memLimit)
			return nil
		},
	}
}

// WithKaiserBestFilter function returns option
// that configures [Resampler] to use the fastest Kaiser filter.
//
// Used by default.
func WithKaiserBestFilter() Option {
	return Option{
		precedence: filterPrecedence,
		apply: func(r *Resampler) error {
			r.f = newFilter(kaiserBestInfo, r.inRate, r.outRate, r.memLimit)
			return nil
		},
	}
}
