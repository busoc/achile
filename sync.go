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
	global, err := SelectHash(*algo)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(client, *algo); err != nil {
		return err
	}
	var (
		count    uint64
		size     float64
		buf      bytes.Buffer
		now      = time.Now()
		local, _ = SelectHash(*algo)
		digest   = io.MultiWriter(global, local)
	)
	for e := range queue {
		if err := e.Compute(digest); err != nil {
			return err
		}
		if *verbose {
			fmt.Fprintf(os.Stdout, "%-8s  %x  %s\n", sizefmt.FormatIEC(e.Size, false), local.Sum(nil), e.File)
		}
		file := strings.TrimPrefix(e.File, cmd.Flag.Arg(1))

		binary.Write(&buf, binary.BigEndian, e.Size)
		buf.Write(global.Sum(nil))
		buf.Write(local.Sum(nil))

		raw := []byte(file)
		binary.Write(&buf, binary.BigEndian, uint16(len(raw)))
		buf.Write(raw)

		if _, err := io.Copy(client, &buf); err != nil {
			return err
		}
		local.Reset()
		count++
		size += e.Size
	}
	if c, ok := client.(*net.TCPConn); ok {
		c.CloseWrite()
	}

	msg := make([]byte, 4096)
	if _, err := client.Read(msg); err != nil {
		return err
	}
	r := bytes.NewReader(msg)
	length, _ := SizeHash(*algo)
	var (
		rcount uint64
		rsize  float64
		rsum   = make([]byte, length)
	)
	binary.Read(r, binary.BigEndian, &rcount)
	binary.Read(r, binary.BigEndian, &rsize)
	if _, err := io.ReadFull(r, rsum); err != nil {
		return err
	}
	if rcount != count {
		return fmt.Errorf("files count mismatched (%d != %d)!", count, rcount)
	}
	if rsize != size {
		return fmt.Errorf("files size mismatched (%s != %s)!", sizefmt.FormatIEC(size, false), sizefmt.FormatIEC(rsize, false))
	}
	if sum := global.Sum(nil); !bytes.Equal(sum, rsum) {
		return fmt.Errorf("files checksum mismatched (%x != %x)!", sum, rsum)
	}
	fmt.Fprintf(os.Stdout, "%s - %d files %x (%s)\n", sizefmt.FormatIEC(size, false), count, global.Sum(nil), time.Since(now))
	return nil
}
