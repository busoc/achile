package main

import (
	"fmt"
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
		fullstat = cmd.Flag.Bool("s", false, "show full stat")
	)
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	scan, err := achile.NewScanner(*algo, *list)
	if err != nil {
		return err
	}
	defer scan.Close()

	now := time.Now()
	cz, err := scan.Scan(cmd.Flag.Arg(0), *pattern, *verbose)
	if err != nil {
		return err
	}
	if *fullstat {
		min, max := cz.Range()
		fmt.Printf("Files  : %d (%x)\n", cz.Count, scan.Checksum())
		fmt.Printf("Size   : %s\n", achile.FormatSize(cz.Size))
		fmt.Printf("Average: %s\n", achile.FormatSize(cz.Avg()))
		fmt.Printf("Range  : %s - %s\n", achile.FormatSize(min), achile.FormatSize(max))
		fmt.Printf("Elapsed: %s\n", time.Since(now))
	} else {
		fmt.Printf("%s - %d files %x (%s)\n", achile.FormatSize(cz.Size), cz.Count, scan.Checksum(), time.Since(now))
	}
	return nil
}
