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

	now := time.Now()
	cz, err := scan.Scan(cmd.Flag.Arg(0), *pattern)
	if err != nil {
		return err
	}
	if elapsed := time.Since(now); *fullstat {
		Full(scan, cz, cmd.Flag.Arg(0), elapsed, *pretty)
	} else {
		Short(scan, cz, elapsed, *pretty)
	}
	return nil
}
