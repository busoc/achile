package main

import (
  "github.com/midbel/sizefmt"
)

func formatSize(z float64) string {
  return sizefmt.FormatIEC(z, false)
}
