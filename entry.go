package achile

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/midbel/glob"
)

type Coze struct {
	Count uint64
	Size  float64

	MinSize float64
	MaxSize float64
}

func (c *Coze) Equal(other Coze) bool {
	return c.Count == other.Count && c.Size == other.Size
}

func (c *Coze) Update(z float64) {
	if z <= 0 {
		return
	}
	if c.Count == 0 || c.MinSize > z {
		c.MinSize = z
	}
	if c.Count == 0 || c.MaxSize < z {
		c.MaxSize = z
	}
	c.Count++
	c.Size += z

}

func (c *Coze) Avg() float64 {
	if c.Count == 0 {
		return 0
	}
	return c.Size / float64(c.Count)
}

func (c *Coze) Range() (float64, float64) {
	return c.MinSize, c.MaxSize
}

type FileInfo struct {
	Size float64
	Accu []byte
	Curr []byte
	Raw  uint16
	File string
}

func FetchInfos(rs io.Reader, length int) <-chan FileInfo {
	queue := make(chan FileInfo)
	go func() {
		defer close(queue)
		for {
			fi := FileInfo{
				Accu: make([]byte, length),
				Curr: make([]byte, length),
			}
			if err := binary.Read(rs, binary.BigEndian, &fi.Size); err != nil || fi.Size == 0 {
				return
			}
			io.ReadFull(rs, fi.Accu)
			io.ReadFull(rs, fi.Curr)
			binary.Read(rs, binary.BigEndian, &fi.Raw)
			file := make([]byte, fi.Raw)
			if _, err := io.ReadFull(rs, file); err != nil {
				return
			}
			fi.File = string(file)
			queue <- fi
		}
	}()
	return queue
}

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
