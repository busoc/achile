package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
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
		Base    string
		Clients uint16 `toml:"client"`
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
		go handle(c, cfg.Base)
	}
	return nil
}

func handle(conn net.Conn, base string) {
	defer conn.Close()

	buf := make([]byte, 16)
	n, err := io.ReadFull(conn, buf)
	if err != nil {
		return
	}
	digest, err := NewDigest(string(bytes.Trim(buf[:n], "\x00")))
	if err1 := reply(conn, err); err != nil || err1 != nil {
		return
	}

	var (
		rs = bufio.NewReader(conn)
		cz Coze
	)
	for {
		req, err := rs.ReadByte()
		if err != nil {
			return
		}
		switch req {
		case ReqCheck:
			err = handleCheck(rs, base, digest)
		case ReqCopy:
			err = handleCopy(rs, base, digest)
		case ReqCmp:
			err = handleCompare(rs, cz, digest)
		default:
			return
		}
		if err := reply(conn, err); err != nil {
			return
		}
	}
}

func reply(w io.Writer, err error) error {
	var buf bytes.Buffer
	switch e := errors.Unwrap(err); e {
	case nil:
		binary.Write(&buf, binary.BigEndian, CodeOk)
	case ErrSize:
		binary.Write(&buf, binary.BigEndian, CodeSize)
	case ErrSum:
		binary.Write(&buf, binary.BigEndian, CodeDigest)
	case ErrFile:
		binary.Write(&buf, binary.BigEndian, CodeNoent)
	default:
		binary.Write(&buf, binary.BigEndian, CodeUnexpected)
	}
	if err != nil {
		io.WriteString(&buf, err.Error())
	}
	_, err = io.Copy(w, &buf)
	return err
}

func handleCheck(rs io.Reader, base string, digest *Digest) error {
	dat := struct {
		Size float64
		Sum  []byte
		Raw  uint16
		File []byte
	}{}

	binary.Read(rs, binary.BigEndian, &dat.Size)
	dat.Sum = make([]byte, digest.Size())
	if _, err := io.ReadFull(rs, dat.Sum); err != nil {
		return err
	}
	binary.Read(rs, binary.BigEndian, &dat.Raw)
	dat.File = make([]byte, dat.Raw)
	if _, err := io.ReadFull(rs, dat.File); err != nil {
		return err
	}

	r, err := os.Open(filepath.Join(base, string(dat.File)))
	if err != nil {
		return ErrFile
	}
	defer r.Close()

	n, err := io.Copy(digest, r)
	if err != nil {
		return err
	}
	if n != int64(dat.Size) {
		return ErrSize
	}
	if !bytes.Equal(dat.Sum, digest.Local()) {
		return ErrSum
	}
	return nil
}

func handleCopy(rs io.Reader, base string, digest *Digest) error {
	return nil
}

func handleCompare(rs io.Reader, cz Coze, digest *Digest) error {
	return nil
}
