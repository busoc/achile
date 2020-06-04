package main

import (
	"github.com/midbel/cli"
)

func main() {
	commands := []*cli.Command{
		{
			Usage: "scan [-a algorithm] [-p pattern] [-w] [-v] <directory>",
			Short: "",
			Alias: []string{"walk"},
			Run:   runScan,
		},
		{
			Usage: "compare [-v] <list> <directory...>",
			Short: "",
			Alias: []string{"cmp"},
			Run:   runCompare,
		},
		{
			Usage: "sync [-a algorithm] [-p pattern] <host:port> <directory>",
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
