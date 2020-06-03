package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/midbel/cli"
	"github.com/midbel/sizefmt"
)

func runScan(cmd *cli.Command, args []string) error {
	var (
		pattern = cmd.Flag.String("p", "", "pattern")
		algo    = cmd.Flag.String("a", "", "algorithm")
		verbose = cmd.Flag.Bool("v", false, "verbose")
		// cmd.Flag.String("w", "", "")
	)
	if err := cmd.Flag.Parse(args); err != nil {
		return err
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
		count    uint64
		size     float64
		now      = time.Now()
		local, _ = SelectHash(*algo)
		digest   = io.MultiWriter(global, local)
	)
	for e := range queue {
		if err := e.Compute(digest); err != nil {
			return err
		}
		if *verbose {
			fmt.Fprintf(os.Stdout, "%-8s  %x  %s\n", sizefmt.FormatIEC(e.Size, false), local.Sum(nil), e.File)
		}
		local.Reset()

		count++
		size += e.Size
	}

	fmt.Fprintf(os.Stdout, "%s - %d files %x (%s)\n", sizefmt.FormatIEC(size, false), count, global.Sum(nil), time.Since(now))
	return nil
}
