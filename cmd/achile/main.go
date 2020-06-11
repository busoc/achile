package main

import (
	"fmt"

	"github.com/busoc/achile"
	"github.com/midbel/cli"
)

const help = `{{.Name}} provides a set of commands to check the integrity of files
after a transfer accross the network.

To check the integrity of files, {{.Name}} supports multiple hashing algorithm:
* MD5
* SHA family (sha1, sha256, sha512,...)
* adler32
* fnv
* xxHash
* murmurhash v3

Usage:

  {{.Name}} command [arguments]

The commands are:

{{range .Commands}}{{printf "  %-9s %s" .String .Short}}
{{end}}

Use {{.Name}} [command] -h for more information about its usage.
`

func main() {
	commands := []*cli.Command{
		{
			Usage: "scan [-a algorithm] [-p pattern] [-w] [-v] <directory>",
			Short: "hash files found in a given directory",
			Alias: []string{"walk"},
			Run:   runScan,
		},
		{
			Usage: "compare [-v] <list> <directory...>",
			Short: "compare files from a list of known hashes",
			Alias: []string{"cmp"},
			Run:   runCompare,
		},
		{
			Usage: "check [-a algorithm] [-p pattern] [-t transfer] <host:port> <directory>",
			Short: "check and compare local files with files on a remote server",
			Run:   runCheck,
		},
		{
			Usage: "transfer [-a algorithm] [-p pattern] <host:port> <directory...>",
			Short: "copy local files in given directory to a remote server",
			Run:   runTransfer,
		},
		{
			Usage: "listen <config>",
			Short: "run a server to verify or copy files from one server to another",
			Alias: []string{"serve"},
			Run:   runListen,
		},
		{
			Usage: "list-hash",
			Short: "print the list of supported hashes",
			Run:   runList,
		},
	}
	cli.RunAndExit(commands, cli.Usage("achile", help, commands))
}

func runList(cmd *cli.Command, args []string) error {
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	fmt.Printf("%-6s %s\n", "size", "algorithm")
	for _, n := range achile.Families {
		z, _ := achile.SizeHash(n)
		fmt.Printf("%-6d %s\n", z, n)
	}
	return nil
}
