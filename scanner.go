package achile

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type Scanner struct {
	closer io.Closer
	inner  *bufio.Writer

	verbose bool
	pretty  bool

	digest *Digest
}

func NewScanner(alg, list string, opts ...Option) (*Scanner, error) {
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

	for _, o := range opts {
		o(&s)
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
	base = filepath.Clean(base)
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
	base = filepath.Clean(base)
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

func (s *Scanner) Scan(base, pattern string) (Coze, error) {
	base = filepath.Clean(base)
	cz, err := s.scanDirectory(base, pattern, func(e Entry) error {
		if err := e.Compute(s.digest); err != nil {
			return err
		}
		if s.verbose {
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
	if s.pretty {
		fmt.Printf("%-8s  %x  %s\n", FormatSize(e.Size), s.digest.Local(), e.File)
	} else {
		fmt.Printf("%-12d  %x  %s\n", int64(e.Size), s.digest.Local(), e.File)
	}
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

func (s *Scanner) setVerbose(v bool) { s.verbose = v }

func (s *Scanner) setPretty(v bool) { s.pretty = v }

func (s *Scanner) setError(v bool) {}
