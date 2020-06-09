package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/midbel/cli"
	"github.com/midbel/sizefmt"
)

func runSync(cmd *cli.Command, args []string) error {
	var (
		pattern = cmd.Flag.String("p", "", "pattern")
		algo    = cmd.Flag.String("a", "", "algorithm")
		verbose = cmd.Flag.Bool("v", false, "verbose")
		// copy    = cmd.Flag.Bool("c", false, "copy")
	)
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	client, err := net.Dial("tcp", cmd.Flag.Arg(0))
	if err != nil {
		return err
	}
	defer client.Close()

	queue, err := FetchFiles(cmd.Flag.Arg(1), *pattern)
	if err != nil {
		return err
	}
	digest, err := NewDigest(*algo)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(client, *algo); err != nil {
		return err
	}
	var (
		cz       Coze
		buf      bytes.Buffer
		now      = time.Now()
	)
	for e := range queue {
		if err := e.Compute(digest); err != nil {
			return err
		}
		if *verbose {
			fmt.Fprintf(os.Stdout, "%-8s  %x  %s\n", sizefmt.FormatIEC(e.Size, false), digest.Local(), e.File)
		}
		file := strings.TrimPrefix(e.File, cmd.Flag.Arg(1))

		binary.Write(&buf, binary.BigEndian, e.Size)
		buf.Write(digest.Global())
		buf.Write(digest.Local())

		raw := []byte(file)
		binary.Write(&buf, binary.BigEndian, uint16(len(raw)))
		buf.Write(raw)

		if _, err := io.Copy(client, &buf); err != nil {
			return err
		}
		digest.Reset()
		cz.Update(e.Size)
	}
	if c, ok := client.(*net.TCPConn); ok {
		c.CloseWrite()
	}

	msg := make([]byte, 4096)
	if _, err := client.Read(msg); err != nil {
		return err
	}
	var (
		resp      = bytes.NewReader(msg)
		rcz       Coze
		rsum      = make([]byte, digest.Size())
	)
	binary.Read(resp, binary.BigEndian, &rcz.Count)
	binary.Read(resp, binary.BigEndian, &rcz.Size)
	if _, err := io.ReadFull(resp, rsum); err != nil {
		return err
	}
	if !cz.Equal(rcz) {
		return fmt.Errorf("files count/sizes mismatched!")
	}
	if sum := digest.Global(); !bytes.Equal(sum, rsum) {
		return fmt.Errorf("files checksum mismatched (%x != %x)!", sum, rsum)
	}
	fmt.Fprintf(os.Stdout, "%s - %d files %x (%s)\n", sizefmt.FormatIEC(cz.Size, false), cz.Count, digest.Global(), time.Since(now))
	return nil
}
