package main

import (
	"fmt"
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
