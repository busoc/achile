# achile: the Haching File tool

achile can be use to compute the global hash of multiple files a little bit like a `find -exec cat | md5sum` but more it provides for each files read their checksums.

```bash
achile provides a set of commands to check the integrity of files
after a transfer accross the network.

To check the integrity of files, achile supports multiple hashing algorithm:
* MD5
* SHA family (sha1, sha256, sha512,...)
* adler32
* fnv
* xxHash
* murmurhash v3

Usage:

  achile command [arguments]

The commands are:

  check     check and compare local files with files on a remote server
  compare   compare files from a list of known hashes
  list-hash print the list of supported hashes
  listen    run a server to verify or copy files from one server to another
  scan      hash files found in a given directory
  transfer  copy local files in given directory to a remote server
```

# building achile

require go version: 1.15

```bash
$ cd path/to/achile/repo
$ go build -o bin/achile cmd/achile
```
