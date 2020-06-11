package main

import (
	"fmt"
	"time"

	"github.com/busoc/achile"
	"github.com/midbel/cli"
)

func runCompare(cmd *cli.Command, args []string) error {
	var (
		list    = cmd.Flag.Bool("l", false, "list")
		verbose = cmd.Flag.Bool("v", false, "verbose")
	)
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	dirs := make([]string, cmd.Flag.NArg()-1)
	for i := 0; i < len(dirs); i++ {
		dirs[i] = cmd.Flag.Arg(i + 1)
	}
	cmp, err := achile.NewComparer(cmd.Flag.Arg(0))
	if err != nil {
		return err
	}
	defer cmp.Close()

	var (
		now = time.Now()
		cz achile.Coze
	)
	if *list {
		cz, err = cmp.List(dirs, *verbose)
	} else {
		cz, err = cmp.Compare(dirs, *verbose)
	}
	if err != nil {
		return err
	}
	fmt.Printf("%s - %d files %x (%s)\n", achile.FormatSize(cz.Size), cz.Count, cmp.Checksum(), time.Since(now))
	return nil
}
