package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
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
	r, err := os.Open(cmd.Flag.Arg(0))
	if err != nil {
		return err
	}
	defer r.Close()

	buf := make([]byte, 16)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	algo := string(bytes.Trim(buf, "\x00"))

	global, err := SelectHash(algo)
	if err != nil {
		return err
	}
	var (
		cz        Coze
		rs        = bufio.NewReaderSize(r, 1<<15)
		now       = time.Now()
		length, _ = SizeHash(algo)
		local, _  = SelectHash(algo)
		digest    = io.MultiWriter(global, local)
	)
	for {
		f := struct {
			Size  float64
			Accu  []byte
			Curr  []byte
			Raw   uint16
			File  string
		}{
			Accu: make([]byte, length),
			Curr: make([]byte, length),
		}
		if err := binary.Read(rs, binary.BigEndian, &f.Size); err != nil || f.Size == 0 {
			break
		}
    io.ReadFull(rs, f.Accu)
    io.ReadFull(rs, f.Curr)
		binary.Read(rs, binary.BigEndian, &f.Raw)
		file := make([]byte, f.Raw)
		if _, err := io.ReadFull(rs, file); err != nil {
			return err
		}
		f.File = string(file)
		var found bool
		for _, d := range dirs {
			file := filepath.Join(d, f.File)
			if s, err := os.Stat(file); err == nil && s.Mode().IsRegular() {
				f.File, found = file, true
				if f.Size != float64(s.Size()) {
					return fmt.Errorf("%s: invalid size (%f != %d)", f.File, f.Size, s.Size())
				}
				break
			}
		}
		if !found {
      return fmt.Errorf("%s: file not found", f.File)
		}
		if err := computeDigest(f.File, digest); err != nil {
			return err
		}
		if !bytes.Equal(local.Sum(nil), f.Curr) && !bytes.Equal(global.Sum(nil), f.Accu) {
			return fmt.Errorf("%s: checksums mismatched", f.File)
		}

		if *verbose {
			fmt.Fprintf(os.Stdout, "%-8s  %x  %s\n", sizefmt.FormatIEC(f.Size, false), local.Sum(nil), f.File)
		}

		cz.Update(f.Size)
		local.Reset()
	}
	fmt.Fprintf(os.Stdout, "%s - %d files %x (%s)\n", sizefmt.FormatIEC(cz.Size, false), cz.Count, global.Sum(nil), time.Since(now))
	return nil
}

func computeDigest(file string, w io.Writer) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	_, err = io.Copy(w, r)
	return err
}
