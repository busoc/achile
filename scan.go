package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/midbel/cli"
	"github.com/midbel/sizefmt"
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
	writer := ioutil.Discard
	if *list != "" {
		w, err := os.Create(*list)
		if err != nil {
			return err
		}
		writer = w
		defer w.Close()

		tmp := make([]byte, 16)
		copy(tmp, *algo)
		if _, err := writer.Write(tmp); err != nil {
			return err
		}
	}

	queue, err := FetchFiles(cmd.Flag.Arg(0), *pattern)
	if err != nil {
		return err
	}
	global, err := SelectHash(*algo)
	if err != nil {
		return err
	}
	var (
		cz       Coze
		ws       = bufio.NewWriter(writer)
		now      = time.Now()
		local, _ = SelectHash(*algo)
		digest   = io.MultiWriter(global, local)
	)
	for e := range queue {
		if err := e.Compute(digest); err != nil {
			return err
		}
		sum := local.Sum(nil)
		if *verbose {
			fmt.Fprintf(os.Stdout, "%-8s  %x  %s\n", sizefmt.FormatIEC(e.Size, false), sum, e.File)
		}
		local.Reset()
		cz.Update(e.Size)

		file := strings.TrimPrefix(e.File, cmd.Flag.Arg(0))
		raw := []byte(file)

		binary.Write(ws, binary.BigEndian, e.Size)
		binary.Write(ws, binary.BigEndian, cz.Size)
		ws.Write(global.Sum(nil))
		ws.Write(sum)
		binary.Write(ws, binary.BigEndian, uint16(len(raw)))
		ws.Write(raw)
	}
	ws.Flush()
	fmt.Fprintf(os.Stdout, "%s - %d files %x (%s)\n", sizefmt.FormatIEC(cz.Size, false), cz.Count, global.Sum(nil), time.Since(now))
	return nil
}
