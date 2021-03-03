package main

import (
	"time"

	"github.com/busoc/achile"
	"github.com/midbel/cli"
)

func runScan(cmd *cli.Command, args []string) error {
	var (
		pattern  = cmd.Flag.String("p", "", "pattern")
		algo     = cmd.Flag.String("a", "", "algorithm")
		list     = cmd.Flag.String("w", "", "file")
		verbose  = cmd.Flag.Bool("v", false, "verbose")
		pretty   = cmd.Flag.Bool("y", false, "pretty size")
		fullstat = cmd.Flag.Bool("s", false, "show full stat")
		zeros    = cmd.Flag.Bool("z", false, "keep results from empty directory")
	)
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	options := []achile.Option{
		achile.WithVerbose(*verbose),
		achile.WithPretty(*pretty),
	}
	scan, err := achile.NewScanner(*algo, *list, options...)
	if err != nil {
		return err
	}
	defer scan.Close()

	var (
		all   achile.Coze
		begin = time.Now()
	)
	for _, a := range cmd.Flag.Args() {
		now := time.Now()
		cz, err := scan.Scan(a, *pattern)
		if err != nil {
			return err
		}
		if !*zeros && cz.Count == 0 {
			continue
		}
		if elapsed := time.Since(now); *fullstat {
			Full(scan, cz, a, elapsed, *pretty)
		} else {
			Short(scan, cz, a, elapsed, *pretty)
		}
		all = all.Merge(cz)
	}
	if cmd.Flag.NArg() > 1 {
		if elapsed := time.Since(begin); *fullstat {
			Full(scan, all, "", elapsed, *pretty)
		} else {
			Short(scan, all, "", elapsed, *pretty)
		}
	}
	return nil
}
