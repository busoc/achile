package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type Scanner struct {
	closer io.Closer
	inner  *bufio.Writer

	digest *Digest
}

func NewScanner(alg, list string) (*Scanner, error) {
	var (
		s Scanner
		w = ioutil.Discard
	)
	if list != "" {
		f, err := os.Create(list)
		if err != nil {
			return nil, err
		}
		s.closer, w = f, f
	}
	s.inner = bufio.NewWriter(w)

	var err error
	if s.digest, err = NewDigest(alg); err != nil {
		return nil, err
	}

	buf := make([]byte, 16)
	copy(buf, alg)
	if _, err := s.inner.Write(buf); err != nil {
		return nil, err
	}
	return &s, nil
}

func (s *Scanner) Checksum() []byte {
	return s.digest.Global()
}

func (s *Scanner) Synchronize(client *Client, base, pattern string, sync, verbose bool) (Coze, error) {
	canCopy := func(err error) bool {
		if !sync {
			return false
		}
		return errors.Is(err, ErrFile) || errors.Is(err, ErrSum) || errors.Is(err, ErrSize)
	}
	cz, err := s.scanDirectory(base, pattern, func(e Entry) error {
		file := e.File
		if err := e.Compute(s.digest); err != nil {
			return err
		}
		e.File = strings.TrimPrefix(e.File, base)
		err := client.Check(e, s.digest.Local())
		if canCopy(err) {
			err = client.Copy(file, e, s.digest.Local())
		}
		if err == nil && verbose {
			s.dumpEntry(e)
		}
		return err
	})
	if err == nil {
		err = client.Compare(cz, s.digest.Global())
	}
	return cz, err
}

func (s *Scanner) Transfer(client *Client, base, pattern string, verbose bool) (Coze, error) {
	cz, err := s.scanDirectory(base, pattern, func(e Entry) error {
		if err := e.Compute(s.digest); err != nil {
			return err
		}
		file := e.File
		e.File = strings.TrimPrefix(e.File, base)
		return client.Copy(file, e, s.digest.Local())
	})
	if err == nil {
		err = client.Compare(cz, s.digest.Global())
	}
	return cz, err
}

func (s *Scanner) Scan(base, pattern string, verbose bool) (Coze, error) {
	cz, err := s.scanDirectory(base, pattern, func(e Entry) error {
		if err := e.Compute(s.digest); err != nil {
			return err
		}
		if verbose {
			s.dumpEntry(e)
		}
		return s.dumpCurrentState(e, base)
	})
	if err == nil {
		err = s.dumpFinalState(cz)
	}
	return cz, err
}

func (s *Scanner) Close() error {
	var err error
	if s.closer != nil {
		err = s.closer.Close()
	}
	return err
}

func (s *Scanner) scanDirectory(base, pattern string, fn func(e Entry) error) (Coze, error) {
	var cz Coze
	queue, err := FetchFiles(base, pattern)
	if err != nil {
		return cz, err
	}
	for e := range queue {
		if err := fn(e); err != nil {
			return cz, err
		}
		cz.Update(e.Size)
		s.digest.Reset()
	}
	return cz, nil
}

func (s *Scanner) dumpEntry(e Entry) {
	fmt.Printf("%-8s  %x  %s\n", formatSize(e.Size), s.digest.Local(), e.File)
}

func (s *Scanner) dumpFinalState(cz Coze) error {
	binary.Write(s.inner, binary.BigEndian, float64(0))
	binary.Write(s.inner, binary.BigEndian, cz.Count)
	binary.Write(s.inner, binary.BigEndian, cz.Size)
	s.inner.Write(s.digest.Global())
	return s.inner.Flush()
}

func (s *Scanner) dumpCurrentState(e Entry, base string) error {
	var (
		file = strings.TrimPrefix(e.File, base)
		raw  = []byte(file)
	)
	binary.Write(s.inner, binary.BigEndian, e.Size)
	s.inner.Write(s.digest.Global())
	s.inner.Write(s.digest.Local())
	binary.Write(s.inner, binary.BigEndian, uint16(len(raw)))
	_, err := s.inner.Write(raw)
	return err
}

type Comparer struct {
	digest *Digest

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
	if c.digest, err = NewDigest(alg); err != nil {
		return nil, err
	}
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
	return c.digest.Global()
}

func (c *Comparer) compareFiles(dirs []string, verbose bool) (Coze, error) {
	var cz Coze
	for i := range FetchInfos(c.inner, c.digest.Size()) {
		fi, found := c.lookupFile(i, dirs)
		if !found {
			break
		}
		if err := c.digestFile(fi); err != nil {
			return cz, err
		}
		if verbose {
			fmt.Printf("%-8s  %x  %s\n", formatSize(fi.Size), c.digest.Local(), fi.File)
		}
		cz.Update(fi.Size)
		c.digest.Reset()
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

	accu := make([]byte, c.digest.Size())
	if _, err := io.ReadFull(c.inner, accu); err != nil {
		return cz, err
	}
	if sum := c.digest.Global(); !bytes.Equal(sum, accu) {
		return z, fmt.Errorf("final checksum mismatchde (%x != %x!)", sum, accu)
	}
	return z, nil
}

func (c *Comparer) lookupFile(fi FileInfo, dirs []string) (FileInfo, bool) {
	var found bool
	for _, d := range dirs {
		file := filepath.Join(d, fi.File)
		if s, err := os.Stat(file); err == nil && s.Mode().IsRegular() {
			fi.File, found = file, true
			break
		}
	}
	return fi, found
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
	if sum := c.digest.Local(); !bytes.Equal(fi.Curr, sum) {
		return fmt.Errorf("%s: checksum mismatched (%x != %x)!", fi.File, fi.Curr, sum)
	}
	if sum := c.digest.Global(); !bytes.Equal(fi.Accu, sum) {
		return fmt.Errorf("%s: checksum mismatched (%x != %x)!", fi.File, fi.Accu, sum)
	}
	return nil
}

type Digest struct {
	global hash.Hash
	local  hash.Hash
	io.Writer
}

func NewDigest(alg string) (*Digest, error) {
	var (
		dgt Digest
		err error
	)
	dgt.global, err = SelectHash(alg)
	if err != nil {
		return nil, err
	}
	dgt.local, _ = SelectHash(alg)
	dgt.Writer = io.MultiWriter(dgt.global, dgt.local)
	return &dgt, nil
}

func (d *Digest) Local() []byte {
	return d.local.Sum(nil)
}

func (d *Digest) Global() []byte {
	return d.global.Sum(nil)
}

func (d *Digest) Size() int {
	return d.global.Size()
}

func (d *Digest) Reset() {
	d.local.Reset()
}

func (d *Digest) ResetAll() {
	d.local.Reset()
	d.global.Reset()
}
