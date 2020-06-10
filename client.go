package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

var (
	ErrMismatch = errors.New("mismatched")
	ErrFile     = errors.New("no such file")
	ErrSum      = errors.New("checksum mismatched")
	ErrSize     = errors.New("file size mismatched")
)

const (
	ReqCheck byte = iota
	ReqCopy
	ReqCmp
)

const (
	CodeOk byte = iota
	CodeDigest
	CodeSize
	CodeNoent
	CodeUnexpected
)

type Client struct {
	conn net.Conn
}

func NewClient(addr, alg string) (*Client, error) {
	c, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	client := Client{conn: c}
	return &client, client.init(alg)
}

func (c *Client) Compare(cz Coze) error {
	return nil
}

func (c *Client) Copy(file string, e Entry, sum []byte) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	var (
		buf bytes.Buffer
		raw = []byte(e.File)
	)
	binary.Write(&buf, binary.BigEndian, ReqCopy)
	binary.Write(&buf, binary.BigEndian, e.Size)
	buf.Write(sum)
	binary.Write(&buf, binary.BigEndian, uint16(len(raw)))
	buf.Write(raw)

	_, err = io.Copy(c.conn, io.MultiReader(&buf, r))
	if err == nil {
		err = c.err()
	}
	return err
}

func (c *Client) Check(e Entry, sum []byte) error {
	var (
		buf bytes.Buffer
		raw = []byte(e.File)
	)
	binary.Write(&buf, binary.BigEndian, ReqCheck)
	binary.Write(&buf, binary.BigEndian, e.Size)
	buf.Write(sum)
	binary.Write(&buf, binary.BigEndian, uint16(len(raw)))
	buf.Write(raw)

	_, err := io.Copy(c.conn, &buf)
	if err == nil {
		err = c.err()
	}
	return err
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) init(alg string) error {
	buf := make([]byte, 16)
	copy(buf, alg)
	if _, err := c.conn.Write(buf); err != nil {
		return err
	}
	return c.err()
}

func (c *Client) err() error {
	c.conn.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 1024)
	n, err := c.conn.Read(buf)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("empty response")
	}

	switch str := bytes.Trim(buf[1:n], "\x00"); buf[0] {
	case CodeOk:
		return nil
	case CodeSize:
		return fmt.Errorf("%w: invalid size", ErrSize)
	case CodeDigest:
		return fmt.Errorf("%w: invalid digest %x", ErrMismatch, str)
	case CodeNoent:
		return fmt.Errorf("%w: no such file on remote %s", ErrFile, str)
	case CodeUnexpected:
		return fmt.Errorf("unexpected error: %s", str)
	default:
		return fmt.Errorf("unexpected error code %02x", buf[0])
	}
}
