package main

import (
	"bytes"
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

var (
	ErrMismatch = errors.New("mismatched")
	ErrFile     = errors.New("no such file")
	ErrSum      = errors.New("checksum mismatched")
	ErrSize     = errors.New("file size mismatched")
	ErrAlg      = errors.New("unsupported algorithm")
)

const (
	ReqCheck byte = iota
	ReqCopy
	ReqCmp
)

const (
	CodeOk uint32 = iota
	CodeDigest
	CodeSize
	CodeNoent
	CodeUnexpected
)

const codeLen = 4 //binary.Size(CodeOk)

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

func (c *Client) Compare(cz Coze, sum []byte) error {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, ReqCmp)
	binary.Write(&buf, binary.BigEndian, cz.Count)
	binary.Write(&buf, binary.BigEndian, cz.Size)
	buf.Write(sum)
	if _, err := io.Copy(c.conn, &buf); err != nil {
		return err
	}
	return c.err()
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
	if n < codeLen {
		return fmt.Errorf("response too short")
	}

	code := binary.BigEndian.Uint32(buf)
	switch str := bytes.Trim(buf[codeLen:n], "\x00"); code {
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
		h.digest.Reset()

		var r *Result
		switch req {
		case ReqCheck:
			r = h.handleCheck(rs)
		case ReqCopy:
			r = h.handleCopy(rs)
		case ReqCmp:
			r = h.handleCompare(rs)
		default:
			r = unhandledResult(fmt.Errorf("unsupported request"))
		}
		if r.IsValid() {
			h.cz.Update(float64(r.Size.Got))
		}
		if err := h.reply(r); err != nil {
			return
		}
	}
}

func (h *Handler) handleCheck(rs io.Reader) *Result {
	dat := struct {
		Size float64
		Sum  []byte
		Raw  uint16
		File []byte
	}{}

	binary.Read(rs, binary.BigEndian, &dat.Size)
	dat.Sum = make([]byte, h.digest.Size())
	if _, err := io.ReadFull(rs, dat.Sum); err != nil {
		return unhandledResult(err)
	}
	binary.Read(rs, binary.BigEndian, &dat.Raw)
	dat.File = make([]byte, dat.Raw)
	if _, err := io.ReadFull(rs, dat.File); err != nil {
		return unhandledResult(err)
	}

	r, err := os.Open(filepath.Join(h.base, string(dat.File)))
	if err != nil {
		return nosuchFileResult(string(dat.File))
	}
	defer r.Close()

	n, err := io.Copy(h.digest, r)
	if err != nil {
		return unhandledResult(err)
	}
	if n != int64(dat.Size) {
		return sizeMismatchResult(string(dat.File), int64(dat.Size), n)
	}
	if sum := h.digest.Local(); !bytes.Equal(dat.Sum, sum) {
		return checksumMismatchResult(string(dat.File), dat.Sum, sum)
	}
	return validResult(string(dat.File), int64(dat.Size), dat.Sum)
}

func (h *Handler) handleCopy(rs io.Reader) *Result {
	dat := struct {
		Size float64
		Sum  []byte
		Raw  uint16
		File []byte
	}{}

	dat.Sum = make([]byte, h.digest.Size())
	binary.Read(rs, binary.BigEndian, &dat.Size)
	if _, err := io.ReadFull(rs, dat.Sum); err != nil {
		return unhandledResult(err)
	}
	binary.Read(rs, binary.BigEndian, &dat.Raw)
	dat.File = make([]byte, dat.Raw)
	if _, err := io.ReadFull(rs, dat.File); err != nil {
		return unhandledResult(err)
	}

	file := filepath.Join(h.base, string(dat.File))
	if err := os.MkdirAll(filepath.Dir(file), 0x755); err != nil {
		return unhandledResult(err)
	}
	w, err := os.Create(file)
	if err != nil {
		return unhandledResult(err)
	}
	defer w.Close()

	n, err := io.CopyN(io.MultiWriter(w, h.digest), rs, int64(dat.Size))
	if err != nil {
		return unhandledResult(err)
	}
	if n != int64(dat.Size) {
		return sizeMismatchResult(string(dat.File), int64(dat.Size), n)
	}
	if sum := h.digest.Local(); !bytes.Equal(dat.Sum, sum) {
		return checksumMismatchResult(string(dat.File), dat.Sum, sum)
	}
	return validResult(string(dat.File), int64(dat.Size), dat.Sum)
}

func (h *Handler) handleCompare(rs io.Reader) *Result {
	var z Coze
	binary.Read(rs, binary.BigEndian, &z.Count)
	binary.Read(rs, binary.BigEndian, &z.Size)
	sum := make([]byte, h.digest.Size())
	if _, err := io.ReadFull(rs, sum); err != nil {
		return unhandledResult(err)
	}

	if !h.cz.Equal(z) {
		return unhandledResult(ErrMismatch)
	}
	if gsum := h.digest.Global(); !bytes.Equal(sum, gsum) {
		return checksumMismatchResult("", sum, gsum)
	}

	return validResult("", int64(z.Size), sum)
}

func (h *Handler) reply(r *Result) error {
	var buf bytes.Buffer
	switch r.Err {
	case nil:
		r.writeOk(&buf)
	case ErrSize:
		r.writeBadSize(&buf)
	case ErrSum:
		r.writeBadSum(&buf)
	case ErrFile:
		r.writeBadFile(&buf)
	default:
		r.writeUnexpected(&buf)
	}
	_, err := io.Copy(h.conn, &buf)
	return err
}

func (h *Handler) init() error {
	buf := make([]byte, 16)
	n, err := io.ReadFull(h.conn, buf)
	if err != nil {
		return err
	}
	h.digest, err = NewDigest(string(bytes.Trim(buf[:n], "\x00")))

	r := emptyResult()
	if err != nil {
		r = unhandledResult(ErrAlg)
	}

	if err1 := h.reply(r); err != nil || err1 != nil {
		if err1 != nil {
			err = err1
		}
		return err
	}
	return nil
}

type Result struct {
	File []byte
	Err  error

	Size struct {
		Want int64
		Got  int64
	}

	Sum struct {
		Want []byte
		Got  []byte
	}
}

func (r Result) IsEmpty() bool {
	return len(r.File) == 0 && r.Err == nil
}

func (r Result) IsValid() bool {
	return r.Err == nil
}

func (r Result) writeOk(w io.Writer) {
	binary.Write(w, binary.BigEndian, CodeOk)
	binary.Write(w, binary.BigEndian, r.Size.Got)
	w.Write(r.Sum.Got)
	r.writeFile(w)
}

func (r Result) writeBadSize(w io.Writer) {
	binary.Write(w, binary.BigEndian, CodeSize)
	binary.Write(w, binary.BigEndian, r.Size.Want)
	binary.Write(w, binary.BigEndian, r.Size.Got)
	r.writeFile(w)
}

func (r Result) writeBadSum(w io.Writer) {
	binary.Write(w, binary.BigEndian, CodeDigest)
	w.Write(r.Sum.Want)
	w.Write(r.Sum.Got)
	r.writeFile(w)
}

func (r Result) writeBadFile(w io.Writer) {
	binary.Write(w, binary.BigEndian, CodeNoent)
	r.writeFile(w)
}

func (r Result) writeUnexpected(w io.Writer) {
	binary.Write(w, binary.BigEndian, CodeUnexpected)
	io.WriteString(w, r.Err.Error())
}

func (r Result) writeFile(w io.Writer) {
	binary.Write(w, binary.BigEndian, uint16(len(r.File)))
	if len(r.File) > 0 {
		w.Write(r.File)
	}
}

func emptyResult() *Result {
	var r Result
	return &r
}

func validResult(file string, size int64, sum []byte) *Result {
	r := Result{File: []byte(file)}
	r.Size.Got = size
	r.Sum.Got = sum

	return &r
}

func unhandledResult(err error) *Result {
	return &Result{Err: err}
}

func nosuchFileResult(file string) *Result {
	return &Result{
		File: []byte(file),
		Err:  ErrFile,
	}
}

func sizeMismatchResult(file string, want, got int64) *Result {
	r := Result{
		File: []byte(file),
		Err:  ErrSize,
	}
	r.Size.Want = want
	r.Size.Got = got

	return &r
}

func checksumMismatchResult(file string, want, got []byte) *Result {
	r := Result{
		File: []byte(file),
		Err:  ErrSum,
	}
	r.Sum.Want = want
	r.Sum.Got = got

	return &r
}
