package main

import (
	"crypto/md5"
  "crypto/sha1"
  "crypto/sha256"
  "crypto/sha512"
  "hash"
  "hash/adler32"
  "hash/fnv"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
  "strings"
	"time"

  "github.com/midbel/xxh"
  "github.com/midbel/glob"
  "github.com/midbel/sizefmt"
  "github.com/midbel/cli"
)

var commands = []*cli.Command{
  {
    Usage: "scan [-v] [-a] [-p] <base>",
    Short: "",
    Run:   runScan,
  },
  {
    Usage: "verify",
    Short: "",
    Run:   runVerify,
  },
}

func main() {
  var (
    verbose = flag.Bool("v", false, "verbose")
    algo = flag.String("a", "", "algorithm")
    pattern = flag.String("p", "", "pattern")
  )
	flag.Parse()

	queue, err := FetchFiles(flag.Arg(0), *pattern)
  if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(2)
  }
  global, err := SelectHash(*algo)
  if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
  }
  var (
    count  int
    size   float64
    now = time.Now();
  )
  for e := range queue {
    local, _ := SelectHash(*algo)
    if err = Process(e, global, local, *verbose); err != nil {
      break
    }
    count++
    size += e.Size
  }
	if err == nil {
		fmt.Fprintf(os.Stdout, "%s - %d files %x (%s)\n", sizefmt.FormatIEC(size, false), count, global.Sum(nil), time.Since(now))
	} else {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(3)
  }
}

func runScan(cmd *cli.Command, args []string) error {
  if err := cmd.Flag.Parse(args); err != nil {
    return err
  }
  return nil
}

func runVerify(cmd *cli.Command, args []string) error {
  if err := cmd.Flag.Parse(args); err != nil {
    return err
  }
  return nil
}

type Entry struct {
  File string
  Size float64
}

func Process(e Entry, global, local hash.Hash, verbose bool) error {
  r, err := os.Open(e.File)
  if err != nil {
    return nil
  }
  defer r.Close()

  z, err := io.CopyBuffer(io.MultiWriter(global, local), r, make([]byte, 32<<10))
  if z != int64(e.Size) {
    return fmt.Errorf("%s: invalid number of bytes copied (%d != %d)", e.File, z, e.Size)
  }
  if err == nil && verbose {
    fmt.Fprintf(os.Stdout, "%-8s %x %s\n", sizefmt.FormatIEC(float64(z), false), local.Sum(nil), e.File)
  }
  return err
}

func FetchFiles(base, pattern string) (<-chan Entry, error) {
	if pattern == "" {
		return walkFiles(base), nil
	}
	return globFiles(base, pattern)
}

func walkFiles(base string) <-chan Entry {
  queue := make(chan Entry)
  go func() {
    defer close(queue)
    filepath.Walk(base, func(file string, i os.FileInfo, err error) error {
      if err != nil || !i.Mode().IsRegular() {
        return nil
      }
      queue <- Entry{
        File: file,
        Size: float64(i.Size()),
      }
      return nil
    })
  }()
  return queue
}

func globFiles(base, pattern string) (<-chan Entry, error) {
  g, err := glob.New(pattern, base)
  if err != nil {
    return nil, err
  }
  queue := make(chan Entry)
  go func() {
    defer close(queue)
    for {
      file := g.Glob()
      if file == "" {
        break
      }
      i, err := os.Stat(file)
      if err == nil && i.Mode().IsRegular() {
        queue <- Entry{
          File: file,
          Size: float64(i.Size()),
        }
      }
    }
  }()
  return queue, nil
}

func SelectHash(alg string) (hash.Hash, error) {
  var (
    h hash.Hash
    err error
  )
  switch strings.ToLower(alg) {
  default:
    err = fmt.Errorf("unsupported hash algorithm")
  case "md5", "":
    h = md5.New()
  case "sha1":
    h = sha1.New()
  case "sha256":
    h = sha256.New()
  case "sha-224":
    h = sha256.New224()
  case "sha512":
    h = sha512.New()
  case "sha-384":
    h = sha512.New384()
  case "adler":
    h = adler32.New()
  case "fnv-32":
    h = fnv.New32()
  case "fnv-64":
    h = fnv.New64()
  case "fnv-128":
    h = fnv.New128()
  case "fnv-32a":
    h = fnv.New32a()
  case "fnv-64a":
    h = fnv.New64a()
  case "fnv-128a":
    h = fnv.New128a()
  case "xxh-32":
    h = xxh.New32(0)
  case "xxh-64":
    h = xxh.New64(0)
  }
  return h, err
}
