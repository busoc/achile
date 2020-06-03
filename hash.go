package main

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"hash/adler32"
	"hash/fnv"
	"strings"

	"github.com/midbel/xxh"
)

const (
	Size32  = 4
	Size64  = 8
	Size128 = 16
)

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
	}
	return z, err
}
