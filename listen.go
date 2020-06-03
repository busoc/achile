package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/midbel/cli"
	"github.com/midbel/toml"
)

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
	var (
		global, _ = SelectHash(algo)
		local, _  = SelectHash(algo)
		digest    = io.MultiWriter(global, local)
		cz        Coze
	)
	for m := range queue {
		var z int64
		for _, d := range dirs {
			file := filepath.Join(d, m.File)
			if s, err := os.Stat(file); err == nil && s.Mode().IsRegular() {
				m.File, z = file, s.Size()
				break
			}
		}
		if z != int64(m.Size) {
			break
		}

		if err := m.Compute(digest); err != nil {
			break
		}
		if sum := local.Sum(nil); !bytes.Equal(sum, m.Curr) {
			break
		}

		cz.Update(m.Size)
		local.Reset()
	}
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, cz.Count)
	binary.Write(&buf, binary.BigEndian, cz.Size)
	buf.Write(global.Sum(nil))

	io.Copy(conn, &buf)
}

type Message struct {
	Entry
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
			file []byte
			raw  uint16
		)

		rs := bufio.NewReaderSize(r, 1<<15)
		for {
			m := Message{
				Accu: make([]byte, length),
				Curr: make([]byte, length),
			}

			if err := binary.Read(rs, binary.BigEndian, &m.Size); err != nil {
				break
			}
			rs.Read(m.Accu)
			rs.Read(m.Curr)

			binary.Read(rs, binary.BigEndian, &raw)
			file = make([]byte, raw)
			if _, err := io.ReadFull(rs, file); err != nil {
				break
			}
			m.File = string(file)

			queue <- m
		}
	}()
	return algo, queue, nil
}
