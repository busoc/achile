package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
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
		Certificate struct{
			Pem string
			Key string
		}
	}{}
	if err := toml.DecodeFile(cmd.Flag.Arg(0), &cfg); err != nil {
		return err
	}
	s, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	if cfg.Certificate.Pem != "" {

	}
	defer s.Close()
	for {
		c, err := s.Accept()
		if err != nil {
			return err
		}
		h, err := NewHandler(c, cfg.Base)
		if err == nil {
			go h.Handle()
		}
	}
	return nil
}

type Handler struct {
	conn   net.Conn
	digest *Digest
	base   string
	cz     Coze
}

func NewHandler(conn net.Conn, base string) (*Handler, error) {
	h := Handler{
		conn: conn,
		base: base,
	}
	return &h, h.init()
}

func (h *Handler) Handle() {
	defer h.conn.Close()

	rs := bufio.NewReader(h.conn)
	for {
		req, err := rs.ReadByte()
		if err != nil {
			return
		}
		var size float64
		switch req {
		case ReqCheck:
			size, err = h.handleCheck(rs)
		case ReqCopy:
			size, err = h.handleCopy(rs)
		case ReqCmp:
			err = h.handleCompare(rs)
		default:
			err = fmt.Errorf("unsupported request")
		}
		h.cz.Update(size)
		if err := h.reply(err); err != nil {
			return
		}
	}
}

func (h *Handler) handleCheck(rs io.Reader) (float64, error) {
	dat := struct {
		Size float64
		Sum  []byte
		Raw  uint16
		File []byte
	}{}

	binary.Read(rs, binary.BigEndian, &dat.Size)
	dat.Sum = make([]byte, h.digest.Size())
	if _, err := io.ReadFull(rs, dat.Sum); err != nil {
		return 0, err
	}
	binary.Read(rs, binary.BigEndian, &dat.Raw)
	dat.File = make([]byte, dat.Raw)
	if _, err := io.ReadFull(rs, dat.File); err != nil {
		return 0, err
	}

	r, err := os.Open(filepath.Join(h.base, string(dat.File)))
	if err != nil {
		return 0, ErrFile
	}
	defer r.Close()

	n, err := io.Copy(h.digest, r)
	if err != nil {
		return 0, err
	}
	if n != int64(dat.Size) {
		return 0, ErrSize
	}
	if !bytes.Equal(dat.Sum, h.digest.Local()) {
		return 0, ErrSum
	}
	return dat.Size, nil
}

func (h *Handler) handleCopy(rs io.Reader) (float64, error) {
	return 0, nil
}

func (h *Handler) handleCompare(rs io.Reader) error {
	var z Coze
	binary.Read(rs, binary.BigEndian, z.Count)
	binary.Read(rs, binary.BigEndian, z.Size)
	sum := make([]byte, h.digest.Size())
	if _, err := io.ReadFull(rs, sum); err != nil {
		return err
	}

	if !h.cz.Equal(z) {
		return ErrMismatch
	}
	if !bytes.Equal(sum, h.digest.Global()) {
		return ErrSum
	}
	return nil
}

func (h *Handler) reply(err error) error {
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
	_, err = io.Copy(h.conn, &buf)
	return err
}

func (h *Handler) init() error {
	buf := make([]byte, 16)
	n, err := io.ReadFull(h.conn, buf)
	if err != nil {
		return err
	}
	h.digest, err = NewDigest(string(bytes.Trim(buf[:n], "\x00")))
	if err1 := h.reply(err); err != nil || err1 != nil {
		if err == nil {
			err = err1
		}
		return err
	}
	return nil
}
