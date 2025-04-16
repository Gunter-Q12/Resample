package resample

const (
	filterPrecedence      = 50
	memoryLimitPrecedence = 100
)

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

type filterInfo struct {
	path     string
	length   int
	density  int
	isScaled bool
}

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

func WithLinearFilter() Option {
	return Option{
		precedence: filterPrecedence,
		apply: func(r *Resampler) error {
			r.f = newFilter(linearInfo, r.inRate, r.outRate, r.memLimit)
			return nil
		},
	}
}

func WithKaiserFastestFilter() Option {
	return Option{
		precedence: filterPrecedence,
		apply: func(r *Resampler) error {
			r.f = newFilter(kaiserFastestInfo, r.inRate, r.outRate, r.memLimit)
			return nil
		},
	}
}

func WithKaiserFastFilter() Option {
	return Option{
		precedence: filterPrecedence,
		apply: func(r *Resampler) error {
			r.f = newFilter(kaiserFastInfo, r.inRate, r.outRate, r.memLimit)
			return nil
		},
	}
}

func WithKaiserBestFilter() Option {
	return Option{
		precedence: filterPrecedence,
		apply: func(r *Resampler) error {
			r.f = newFilter(kaiserBestInfo, r.inRate, r.outRate, r.memLimit)
			return nil
		},
	}
}
