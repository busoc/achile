package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"net"
	"strings"
	"time"

	"github.com/midbel/cli"
)

func runScan(cmd *cli.Command, args []string) error {
	var (
		pattern = cmd.Flag.String("p", "", "pattern")
		algo    = cmd.Flag.String("a", "", "algorithm")
		list    = cmd.Flag.String("w", "", "file")
		verbose = cmd.Flag.Bool("v", false, "verbose")
	)
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	scan, err := NewScanner(*algo, *list)
	if err != nil {
		return err
	}
	defer scan.Close()

	now := time.Now()
	cz, err := scan.Scan(cmd.Flag.Arg(0), *pattern, *verbose)
	if err != nil {
		return err
	}
	fmt.Printf("%s - %d files %x (%s)\n", formatSize(cz.Size), cz.Count, scan.Checksum(), time.Since(now))
	return nil
}

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

// func (s *Scanner) Synchronize(conn net.Conn, base, pattern string) (Coze, error) {
// 	return s.scanDirectory(base, pattern, func(e Entry) error {
// 		return nil
// 	})
// }

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
