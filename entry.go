package main

import (
  "os"
  "io"
  "fmt"
  "path/filepath"

  "github.com/midbel/glob"
)

type Entry struct {
	File string
	Size float64
}

func (e Entry) Compute(w io.Writer) error {
	r, err := os.Open(e.File)
	if err != nil {
		return err
	}
	defer r.Close()

	z, err := io.Copy(w, r)
	if err != nil {
		return err
	}
	if z != int64(e.Size) {
		err = fmt.Errorf("invalid number of bytes copied (%d != %d)", z, e.Size)
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
			if err != nil || !i.Mode().IsRegular() || i.Size() <= 0 {
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
			if err == nil && i.Mode().IsRegular() && i.Size() > 0 {
				queue <- Entry{
					File: file,
					Size: float64(i.Size()),
				}
			}
		}
	}()
	return queue, nil
}
