package achile

import (
	"github.com/midbel/sizefmt"
)

func FormatSize(z float64) string {
	return sizefmt.FormatIEC(z, false)
}
