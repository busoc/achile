package main

import (
	"strings"
	"time"

	"github.com/busoc/achile"
	"github.com/midbel/cli"
)

func runCompare(cmd *cli.Command, args []string) error {
	var (
		list   = cmd.Flag.Bool("l", false, "list")
		pretty = cmd.Flag.Bool("y", false, "pretty size")
		// abort    = cmd.Flag.Bool("e", false, "")
		verbose  = cmd.Flag.Bool("v", false, "verbose")
		fullstat = cmd.Flag.Bool("s", false, "show full stats")
	)
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	dirs := make([]string, cmd.Flag.NArg()-1)
	for i := 0; i < len(dirs); i++ {
		dirs[i] = cmd.Flag.Arg(i + 1)
	}
	options := []achile.Option{
		achile.WithPretty(*pretty),
		achile.WithVerbose(*verbose),
	}
	cmp, err := achile.NewComparer(cmd.Flag.Arg(0), options...)
	if err != nil {
		return err
	}
	defer cmp.Close()

	var (
		now = time.Now()
		cz  achile.Coze
	)
	if *list {
		cz, err = cmp.List(dirs)
	} else {
		cz, err = cmp.Compare(dirs)
	}
	if elapsed, dir := time.Since(now), strings.Join(dirs, ", "); *fullstat {
		Full(cmp, cz, dir, elapsed, *pretty)
	} else {
		Short(cmp, cz, dir, elapsed, *pretty)
	}
	return err
}
