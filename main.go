package main

import (
	"fmt"
	"os"
)

const usage = `boltbackup is file backup utility, built on boltdb

Usage:

	boltbackup command [arguments]

The commands are:

	backup      put a bunch of files into a boltdb file
	restore     get files out of a db
	ls          list files that exist in a db


See also:
	boltdb          https://github.com/boltdb/bolt
	pattern syntax  https://golang.org/pkg/path/filepath/#Match
`

var bucketName = []byte("_bucket") // TODO better name?

func main() {
	if len(os.Args) < 2 {
		fmt.Println(usage)
		os.Exit(0)
	}
	subcmdArgs := os.Args[2:]
	switch os.Args[1] {
	case "backup":
		backup(subcmdArgs)
	case "restore":
		restore(subcmdArgs)
	case "ls":
		ls(subcmdArgs)
	default:
		fmt.Println(usage)
		os.Exit(0)
	}
}
