# Resample

[![Go Reference](https://pkg.go.dev/badge/github.com/Gunter-Q12/Resample.svg)](https://pkg.go.dev/github.com/Gunter-Q12/Resample)
[![Go Report Card](https://goreportcard.com/badge/github.com/Gunter-Q12/Resample)](https://goreportcard.com/report/github.com/Gunter-Q12/Resample)
[![cov](https://Gunter-Q12.github.io/Resample/badges/coverage.svg)](https://github.com/Gunter-Q12/Resample/actions)

Resample is a package for audio resampling in pure Go.
Implementation is based on [bandlimited interpolation](https://ccrma.stanford.edu/~jos/resample/resample.pdf).

**Resample's main features are:**

- io.Copy support (uses little RAM, even for GB-sized files)
- concurrency
- speed
- tested accuracy
- no C dependencies

## Example
See the [API documentation on go.dev](https://pkg.go.dev/github.com/gunter-q12/resample).

```go
package main

import (
	"github.com/gunter-q12/resample"
	"io"
	"os"
)

func main() {
	input, _ := os.Open("./original.raw")
	output, _ := os.Create("./resampled.raw")

	res, _ := resample.New(output, resample.FormatInt16, 48000, 16000, 2)
	_, _ = io.Copy(res, input)
}
```

## Benchmarks
### Precision
Precision test were performed againstx
a well-known [SOXR](https://github.com/chirlu/soxr) library
using a Go wrapper by [zaf](https://github.com/zaf/resample).

**TODO**

### Performance

**TODO**
