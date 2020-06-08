package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/midbel/cli"
	"github.com/midbel/sizefmt"
)

func runCompare(cmd *cli.Command, args []string) error {
	verbose := cmd.Flag.Bool("v", false, "verbose")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	dirs := make([]string, cmd.Flag.NArg()-1)
	for i := 0; i < len(dirs); i++ {
		dirs[i] = cmd.Flag.Arg(i + 1)
	}
	cmp, err := NewComparer(cmd.Flag.Arg(0))
	if err != nil {
		return err
	}
	defer cmp.Close()

	now := time.Now()
	cz, err := cmp.Compare(dirs, *verbose)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s - %d files %x (%s)\n", sizefmt.FormatIEC(cz.Size, false), cz.Count, cmp.Checksum(), time.Since(now))
	return nil
}

type Comparer struct {
	global hash.Hash
	local  hash.Hash
	digest io.Writer

	inner *bufio.Reader
	io.Closer
}

func NewComparer(file string) (*Comparer, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	var (
		buf = make([]byte, 16)
		alg string
	)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	alg = string(bytes.Trim(buf, "\x00"))

	var c Comparer
	if c.global, err = SelectHash(alg); err != nil {
		return nil, err
	}
	c.local, _ = SelectHash(alg)

	c.digest = io.MultiWriter(c.global, c.local)
	c.inner = bufio.NewReader(r)
	c.Closer = r

	return &c, nil
}

func (c *Comparer) Compare(dirs []string, verbose bool) (Coze, error) {
	cz, err := c.compareFiles(dirs, verbose)
	if err == nil {
		cz, err = c.compare(cz)
	}
	return cz, err
}

func (c *Comparer) Checksum() []byte {
	return c.global.Sum(nil)
}

func (c *Comparer) compareFiles(dirs []string, verbose bool) (Coze, error) {
	var cz Coze
	for fi := range FetchInfos(c.inner, c.global.Size()) {
		var found bool
		for _, d := range dirs {
			file := filepath.Join(d, fi.File)
			if s, err := os.Stat(file); err == nil && s.Mode().IsRegular() {
				fi.File, found = file, true
				break
			}
		}
		if !found {
			break
		}
		if verbose {
			fmt.Fprintf(os.Stdout, "%-8s  %x  %s\n", sizefmt.FormatIEC(fi.Size, false), c.local.Sum(nil), fi.File)
		}
		if err := c.digestFile(fi); err != nil {
			return cz, err
		}
		cz.Update(fi.Size)
		c.local.Reset()
	}
	return cz, nil
}

func (c *Comparer) compare(cz Coze) (Coze, error) {
	var z Coze
	binary.Read(c.inner, binary.BigEndian, &z.Count)
	binary.Read(c.inner, binary.BigEndian, &z.Size)
	if !cz.Equal(z) {
		return z, fmt.Errorf("final count/size mismatched!")
	}

	accu := make([]byte, c.global.Size())
	if _, err := io.ReadFull(c.inner, accu); err != nil {
		return cz, err
	}
	if sum := c.global.Sum(nil); !bytes.Equal(sum, accu) {
		return z, fmt.Errorf("final checksum mismatchde (%x != %x!)", sum, accu)
	}
	return z, nil
}

func (c *Comparer) digestFile(fi FileInfo) error {
	r, err := os.Open(fi.File)
	if err != nil {
		return err
	}
	defer r.Close()

	n, err := io.Copy(c.digest, r)
	if err != nil {
		return err
	}
	if n != int64(fi.Size) {
		return fmt.Errorf("%s: size mismatched (%f != %d)!", fi.Size, fi.Size, n)
	}
	if sum := c.local.Sum(nil); !bytes.Equal(fi.Curr, sum) {
		return fmt.Errorf("%s: checksum mismatched (%x != %x)!", fi.File, fi.Curr, sum)
	}
	if sum := c.global.Sum(nil); !bytes.Equal(fi.Accu, sum) {
		return fmt.Errorf("%s: checksum mismatched (%x != %x)!", fi.File, fi.Accu, sum)
	}
	return nil
}
