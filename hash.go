package achile

import (
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
	"sort"
	"strings"

	"github.com/midbel/murmur"
	"github.com/midbel/xxh"
)

const (
	Size32  = 4
	Size64  = 8
	Size128 = 16
)

var Families = []string{
	"md5",
	"sha1",
	"sha256",
	"sha224",
	"sha512",
	"sha384",
	"xxh32",
	"xxh64",
	"adler",
	"fnv32",
	"fnv32a",
	"fnv64",
	"fnv64a",
	"fnv128",
	"fnv128a",
	"murmur32",
	"murmur128x86",
	"murmur128x64",
	"none",
	"sum32",
	"sum64",
}

func init() {
	sort.Strings(Families)
}

type Digest struct {
	global hash.Hash
	local  hash.Hash
	io.Writer
}

func NewDigest(alg string) (*Digest, error) {
	var (
		dgt Digest
		err error
	)
	dgt.global, err = SelectHash(alg)
	if err != nil {
		return nil, err
	}
	dgt.local, _ = SelectHash(alg)
	dgt.Writer = io.MultiWriter(dgt.global, dgt.local)
	return &dgt, nil
}

func (d *Digest) Local() []byte {
	return d.local.Sum(nil)
}

func (d *Digest) Global() []byte {
	return d.global.Sum(nil)
}

func (d *Digest) Size() int {
	return d.global.Size()
}

func (d *Digest) Reset() {
	d.local.Reset()
}

func (d *Digest) ResetAll() {
	d.local.Reset()
	d.global.Reset()
}

func SelectHash(alg string) (hash.Hash, error) {
	var (
		h   hash.Hash
		err error
	)
	switch strings.ToLower(alg) {
	default:
		err = fmt.Errorf("unsupported hash algorithm")
	case "sum32":
		h = Sum32()
	case "sum64":
		h = Sum64()
	case "none":
		h = None()
	case "md5", "":
		h = md5.New()
	case "sha1":
		h = sha1.New()
	case "sha256":
		h = sha256.New()
	case "sha224":
		h = sha256.New224()
	case "sha512":
		h = sha512.New()
	case "sha384":
		h = sha512.New384()
	case "adler":
		h = adler32.New()
	case "fnv32":
		h = fnv.New32()
	case "fnv64":
		h = fnv.New64()
	case "fnv128":
		h = fnv.New128()
	case "fnv32a":
		h = fnv.New32a()
	case "fnv64a":
		h = fnv.New64a()
	case "fnv128a":
		h = fnv.New128a()
	case "xxh32":
		h = xxh.New32(0)
	case "xxh64":
		h = xxh.New64(0)
	case "murmur32":
		h = murmur.Murmur32x86v3(0)
	case "murmur128x86":
		h = murmur.Murmur128x86v3(0)
	case "murmur128x64":
		h = murmur.Murmur128x64v3(0)
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
	case "sum32":
		z = Size32
	case "sum64":
		z = Size64
	case "none":
		z = Size32
	case "md5", "":
		z = md5.Size
	case "sha1":
		z = sha1.Size
	case "sha256":
		z = sha256.Size
	case "sha224":
		z = sha256.Size224
	case "sha512":
		z = sha512.Size
	case "sha384":
		z = sha512.Size384
	case "adler":
		z = Size32
	case "fnv32":
		z = Size32
	case "fnv64":
		z = Size64
	case "fnv128":
		z = Size128
	case "fnv32a":
		z = Size32
	case "fnv64a":
		z = Size64
	case "fnv128a":
		z = Size128
	case "xxh32":
		z = Size32
	case "xxh64":
		z = Size64
	case "murmur32":
		z = Size32
	case "murmur128x86":
		z = Size128
	case "murmur128x64":
		z = Size128
	}
	return z, err
}

type none struct {}

func None() hash.Hash {
	return none{}
}

func (n none) Reset() {}

func (n none) Write(bs []byte) (int, error) {
	return len(bs), nil
}

func (n none) Sum(bs []byte) []byte {
	return make([]byte, 4)
}

func (n none) Size() int {
	return Size32
}

func (n none) BlockSize() int {
	return Size32
}

type sum32 uint32

func Sum32() hash.Hash {
	var s sum32;
	s.Reset()
	return &s
}

func (s *sum32) Reset() {
	*s -= *s
}

func (s *sum32) Write(bs []byte) (int, error) {
	for _, b := range bs {
		*s += sum32(b)
	}
	return len(bs), nil
}

func (s *sum32) Sum(bs []byte) []byte {
	xs := make([]byte, Size32)
	binary.BigEndian.PutUint32(xs, uint32(*s))
	return append(bs, xs...)
}

func (s *sum32) Size() int {
	return Size32
}

func (s *sum32) BlockSize() int {
	return Size32
}

type sum64 uint64

func Sum64() hash.Hash {
	var s sum64;
	s.Reset()
	return &s
}

func (s *sum64) Reset() {
	*s -= *s
}

func (s *sum64) Write(bs []byte) (int, error) {
	for _, b := range bs {
		*s += sum64(b)
	}
	return len(bs), nil
}

func (s *sum64) Sum(bs []byte) []byte {
	xs := make([]byte, Size64)
	binary.BigEndian.PutUint64(xs, uint64(*s))
	return append(bs, xs...)
}

func (s *sum64) Size() int {
	return Size64
}

func (s *sum64) BlockSize() int {
	return Size64
}
