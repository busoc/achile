package main

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"hash"
	"hash/adler32"
	"hash/fnv"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/midbel/cli"
	"github.com/midbel/glob"
	"github.com/midbel/sizefmt"
	"github.com/midbel/toml"
	"github.com/midbel/xxh"
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

func runScan(cmd *cli.Command, args []string) error {
	var (
		pattern = cmd.Flag.String("p", "", "pattern")
		algo    = cmd.Flag.String("a", "", "algorithm")
		verbose = cmd.Flag.Bool("v", false, "verbose")
	)
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	queue, err := FetchFiles(cmd.Flag.Arg(0), *pattern)
	if err != nil {
		return err
	}
	global, err := SelectHash(*algo)
	if err != nil {
		return err
	}
	var (
		count uint64
		size  float64
		now   = time.Now()
	)
	local, _ := SelectHash(*algo)
	for e := range queue {
		if err := e.Compute(io.MultiWriter(global, local)); err != nil {
			return err
		}
		if *verbose {
			fmt.Fprintf(os.Stdout, "%-8s  %x  %s\n", sizefmt.FormatIEC(e.Size, false), local.Sum(nil), e.File)
		}
		local.Reset()

		count++
		size += e.Size
	}

	fmt.Fprintf(os.Stdout, "%s - %d files %x (%s)\n", sizefmt.FormatIEC(size, false), count, global.Sum(nil), time.Since(now))
	return nil
}

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
		count uint64
		size  float64
		buf   bytes.Buffer
		now   = time.Now()
	)
	local, _ := SelectHash(*algo)
	for e := range queue {
		if err := e.Compute(io.MultiWriter(global, local)); err != nil {
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

func runListen(cmd *cli.Command, args []string) error {
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	cfg := struct {
		Addr    string
		Clients uint16   `toml:"client"`
		Bases   []string `toml:"base"`
	}{}
	if err := toml.DecodeFile(cmd.Flag.Arg(0), &cfg); err != nil {
		return err
	}
	s, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	defer s.Close()
	for {
		c, err := s.Accept()
		if err != nil {
			return err
		}
		go handle(c, cfg.Bases)
	}
	return nil
}

func handle(conn net.Conn, dirs []string) {
	defer conn.Close()
	algo, queue, err := FetchMessages(conn)
	if err != nil {
		return
	}
	global, _ := SelectHash(algo)
	local, _ := SelectHash(algo)
	var (
		count uint64
		size  float64
	)
	for m := range queue {
		var found bool
		for _, d := range dirs {
			file := filepath.Join(d, m.File)
			s, err := os.Stat(file)
			if found = err == nil && s.Mode().IsRegular(); found {
				m.File = file
				break
			}
		}
		if !found {
			continue
		}

		if err := m.Compute(io.MultiWriter(global, local)); err != nil {
			return
		}
		if sum := local.Sum(nil); !bytes.Equal(sum, m.Curr) {
			return
		}
		count++
		size += m.Size
		local.Reset()
	}
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, count)
	binary.Write(&buf, binary.BigEndian, size)
	buf.Write(global.Sum(nil))

	io.Copy(conn, &buf)
}

type Message struct {
	Entry
	Algo string
	Accu []byte
	Curr []byte
}

func FetchMessages(r io.Reader) (string, <-chan Message, error) {
	buf := make([]byte, 1<<14)
	n, err := r.Read(buf)
	if err != nil {
		return "", nil, err
	}
	algo := string(buf[:n])
	length, err := SizeHash(algo)
	if err != nil {
		return "", nil, err
	}
	queue := make(chan Message)
	go func() {
		defer close(queue)

		var (
			tmp  = bytes.NewReader(nil)
			file []byte
			raw  uint16
		)

		for {
			n, err := r.Read(buf)
			if err != nil || n == 0 {
				return
			}
			tmp.Reset(buf[:n])
			for tmp.Len() > 0 {
				m := Message{
					Accu: make([]byte, length),
					Curr: make([]byte, length),
				}

				binary.Read(tmp, binary.BigEndian, &m.Size)
				tmp.Read(m.Accu)
				tmp.Read(m.Curr)

				binary.Read(tmp, binary.BigEndian, &raw)
				file = make([]byte, raw)
				tmp.Read(file)
				m.File = string(file)
				fmt.Println(m.File, raw, tmp.Size(), tmp.Len())

				queue <- m
			}
		}
	}()
	return algo, queue, nil
}

type Entry struct {
	File string
	Size float64
}

func (e Entry) Compute(w io.Writer) error {
	r, err := os.Open(e.File)
	if err != nil {
		return err
	}
	defer r.Close()

	z, err := io.Copy(w, r)
	if err != nil {
		return err
	}
	if z != int64(e.Size) {
		err = fmt.Errorf("invalid number of bytes copied (%d != %d)", z, e.Size)
	}
	return err
}

func FetchFiles(base, pattern string) (<-chan Entry, error) {
	if pattern == "" {
		return walkFiles(base), nil
	}
	return globFiles(base, pattern)
}

func walkFiles(base string) <-chan Entry {
	queue := make(chan Entry)
	go func() {
		defer close(queue)
		filepath.Walk(base, func(file string, i os.FileInfo, err error) error {
			if err != nil || !i.Mode().IsRegular() || i.Size() <= 0 {
				return nil
			}
			queue <- Entry{
				File: file,
				Size: float64(i.Size()),
			}
			return nil
		})
	}()
	return queue
}

func globFiles(base, pattern string) (<-chan Entry, error) {
	g, err := glob.New(pattern, base)
	if err != nil {
		return nil, err
	}
	queue := make(chan Entry)
	go func() {
		defer close(queue)
		for {
			file := g.Glob()
			if file == "" {
				break
			}
			i, err := os.Stat(file)
			if err == nil && i.Mode().IsRegular() && i.Size() > 0 {
				queue <- Entry{
					File: file,
					Size: float64(i.Size()),
				}
			}
		}
	}()
	return queue, nil
}

func SelectHash(alg string) (hash.Hash, error) {
	var (
		h   hash.Hash
		err error
	)
	switch strings.ToLower(alg) {
	default:
		err = fmt.Errorf("unsupported hash algorithm")
	case "md5", "":
		h = md5.New()
	case "sha1":
		h = sha1.New()
	case "sha256":
		h = sha256.New()
	case "sha-224":
		h = sha256.New224()
	case "sha512":
		h = sha512.New()
	case "sha-384":
		h = sha512.New384()
	case "adler":
		h = adler32.New()
	case "fnv-32":
		h = fnv.New32()
	case "fnv-64":
		h = fnv.New64()
	case "fnv-128":
		h = fnv.New128()
	case "fnv-32a":
		h = fnv.New32a()
	case "fnv-64a":
		h = fnv.New64a()
	case "fnv-128a":
		h = fnv.New128a()
	case "xxh-32":
		h = xxh.New32(0)
	case "xxh-64":
		h = xxh.New64(0)
	}
	return h, err
}

func SizeHash(alg string) (int, error) {
	var (
		z   int
		err error
	)
	switch strings.ToLower(alg) {
	default:
		err = fmt.Errorf("unsupported hash algorithm")
	case "md5", "":
		z = md5.Size
	case "sha1":
		z = sha1.Size
	case "sha256":
		z = sha256.Size
	case "sha-224":
		z = sha256.Size224
	case "sha512":
		z = sha512.Size
	case "sha-384":
		z = sha512.Size384
	case "adler":
		z = 4
	case "fnv-32":
		z = 4
	case "fnv-64":
		z = 8
	case "fnv-128":
		z = 32
	case "fnv-32a":
		z = 4
	case "fnv-64a":
		z = 8
	case "fnv-128a":
		z = 32
	case "xxh-32":
		z = 4
	case "xxh-64":
		z = 8
	}
	return z, err
}
