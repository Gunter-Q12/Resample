package resample

const (
	filterPrecedence      = 50
	memoizationPrecedence = 100
)

// Option is a struct used to configure [Resampler].
type Option struct {
	precedence int
	apply      func(*Resampler) error
}

func optionCmp(a, b Option) int {
	return b.precedence - a.precedence
}

// WithNoMemoization function returns option that disables memoization in [Resampler].
//
// This option should be used when [Resampler] consumers too much memory.
// Such behaviour may occur when resampling between
// sampling rages with a small greatest common divisor (e.g. 9999 and 10000).
//
// Enabling this function slows the resampling progress significantly.
// Therefore, Most users should avoid it and should switch used filter instead.
func WithNoMemoization() Option {
	return Option{
		precedence: memoizationPrecedence,
		apply: func(r *Resampler) error {
			r.memoization = false
			return nil
		},
	}
}

// fileInfo stores info about precompiled filters.
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
	return withFilter(linearInfo)
}

// WithKaiserFastestFilter function returns option
// that configures [Resampler] to use the fast Kaiser filter.
//
// Fastest Kaiser filter provides higher resampling speed in exchange for lower quality.
func WithKaiserFastestFilter() Option {
	return withFilter(kaiserFastestInfo)
}

// WithKaiserFastFilter function returns option
// that configures [Resampler] to use the best Kaiser filter.
//
// Best Kaiser filter provides higher resampling speed in exchange for lower quality.
func WithKaiserFastFilter() Option {
	return withFilter(kaiserFastInfo)
}

// WithKaiserBestFilter function returns option
// that configures [Resampler] to use the fastest Kaiser filter.
//
// Used by default.
func WithKaiserBestFilter() Option {
	return withFilter(kaiserBestInfo)
}

// withFilter is an actual implementation for all WithFilterX functions.
func withFilter(info filterInfo) Option {
	return Option{
		precedence: filterPrecedence,
		apply: func(r *Resampler) error {
			r.f = newFilter(info, r.inRate, r.outRate, r.memoization)
			return nil
		},
	}
}
