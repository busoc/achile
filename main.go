package main

import (
	"github.com/midbel/cli"
)

func main() {
	commands := []*cli.Command{
		{
			Usage: "scan [-q] [-a] [-p] <base>",
			Short: "",
			Run:   runScan,
		},
		{
			Usage: "sync [-c] [-a] [-p] <remote> <base>",
			Short: "",
			Run:   runSync,
		},
		{
			Usage: "listen <config>",
			Short: "",
			Run:   runListen,
		},
	}
	cli.RunAndExit(commands, func() {})
}
