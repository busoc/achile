package main

import (
  "bytes"
  "encoding/binary"
  "net"
)

type Client struct {
  net.Conn
  buf bytes.Buffer
}

func NewClient(addr string) (*Client, error) {
  c, err := net.Dial("tcp", addr)
  if err != nil {
    return nil, err
  }
  return &Client{c}, nil
}

func (c *Client) Submit(fi FileInfo) error {
  if err := c.submit(fi); err != nil {
    return err
  }
  return c.readResponse()
}

func (c *Client) submit(fi FileInfo) error {
  binary.Write(&c.buf, binary.BigEndian, fi.Size)
  c.buf.Write(fi.Accu)
  c.buf.Write(fi.Curr)

  raw := []byte(fi.File)
  binary.Write(&c.buf, binary.BigEndian, uint16(len(raw)))
  c.buf.Write(raw)

  _, err := io.Copy(c.Conn, &c.buf)
  return err
}

func (c *Client) readResponse() error {
  buf := make([]byte, 16)
  n, err := c.Read(buf)
  if err != nil {
    return err
  }
  if n == 0 {
    return nil
  }
  return nil
}
