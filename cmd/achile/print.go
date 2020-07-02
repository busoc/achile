package main

import (
  "fmt"
  "time"

  "github.com/busoc/achile"
)

type Checksumer interface {
  Checksum() []byte
}

func Full(ac Checksumer, cz achile.Coze, base string, elapsed time.Duration, pretty bool) {
  min, max := cz.Range()
  fmt.Printf("Directory: %s\n", base)
  fmt.Printf("Files    : %d (%x)\n", cz.Count, ac.Checksum())
  if pretty {
    fmt.Printf("Size     : %s\n", achile.FormatSize(cz.Size))
    fmt.Printf("Average  : %s\n", achile.FormatSize(cz.Avg()))
    fmt.Printf("Range    : %s - %s\n", achile.FormatSize(min), achile.FormatSize(max))
  } else {
    fmt.Printf("Size     : %d\n", int64(cz.Size))
    fmt.Printf("Average  : %d\n", int64(cz.Avg()))
    fmt.Printf("Range    : %d - %d\n", int64(min), int64(max))
  }
  fmt.Printf("Elapsed  : %s\n", elapsed)
}

func Short(ac Checksumer, cz achile.Coze, elapsed time.Duration, pretty bool) {
  if sum := ac.Checksum(); pretty {
    fmt.Printf("%s - %d files %x (%s)\n", achile.FormatSize(cz.Size), cz.Count, sum, elapsed)
  } else {
    fmt.Printf("%d - %d files %x (%s)\n", int64(cz.Size), cz.Count, sum, elapsed)
  }
}
