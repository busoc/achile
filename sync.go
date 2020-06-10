package main

import (
	"fmt"
	"time"

	"github.com/midbel/cli"
)

func runTransfer(cmd *cli.Command, args []string) error {
	var (
		pattern = cmd.Flag.String("p", "", "pattern")
		algo    = cmd.Flag.String("a", "", "algorithm")
		verbose = cmd.Flag.Bool("v", false, "verbose")
	)
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	client, err := NewClient(cmd.Flag.Arg(0), *algo)
	if err != nil {
		return err
	}
	defer client.Close()

	scan, err := NewScanner(*algo, "")
	if err != nil {
		return err
	}
	defer scan.Close()

	now := time.Now()
	for i := 1; i < cmd.Flag.NArg(); i++ {
		cz, err := scan.Transfer(client, cmd.Flag.Arg(i), *pattern, *verbose)
		if err != nil {
			return err
		}
		fmt.Printf("%s - %d files %x (%s)\n", formatSize(cz.Size), cz.Count, scan.Checksum(), time.Since(now))
	}
	return nil
}

func runSync(cmd *cli.Command, args []string) error {
	var (
		pattern = cmd.Flag.String("p", "", "pattern")
		algo    = cmd.Flag.String("a", "", "algorithm")
		verbose = cmd.Flag.Bool("v", false, "verbose")
		copy    = cmd.Flag.Bool("s", false, "synchronize")
	)
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	client, err := NewClient(cmd.Flag.Arg(0), *algo)
	if err != nil {
		return err
	}
	defer client.Close()

	scan, err := NewScanner(*algo, "")
	if err != nil {
		return err
	}
	defer scan.Close()

	now := time.Now()
	cz, err := scan.Synchronize(client, cmd.Flag.Arg(1), *pattern, *copy, *verbose)
	if err != nil {
		return err
	}
	fmt.Printf("%s - %d files %x (%s)\n", formatSize(cz.Size), cz.Count, scan.Checksum(), time.Since(now))
	return nil
}
