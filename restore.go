package main

import (
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/boltdb/bolt"
	"github.com/fatih/color"
)

func restore(arguments []string) {
	var (
		// Flags for this subcommand
		flags = flag.NewFlagSet("restore", flag.ExitOnError)

		outdir = flags.String("o", ".",
			"directory to write files to")

		dbpath = flags.String("db", "",
			"boltdb file")
	)
	if err := flags.Parse(arguments); err != nil {
		color.Red("error parsing flags: %v", err)
		os.Exit(1)
	}

	// Flag checking
	var err error
	*outdir, err = filepath.Abs(*outdir)
	if err != nil {
		color.Red("error creating absolute path for %s: %v", *outdir, err)
		os.Exit(1)
	}
	switch info, err := os.Stat(*outdir); {
	case err != nil:
		color.Red("error reading -o directory: %v", err)
		os.Exit(1)
	case !info.IsDir():
		color.Red("-o is not a directory")
		os.Exit(1)
	}
	if *dbpath == "" {
		color.Red("-db missing: need a boltdb path")
		os.Exit(1)
	}

	// Open the database
	db, err := bolt.Open(*dbpath, 0600, nil)
	if err != nil {
		color.Red("error opening boltdb: %v", err)
		os.Exit(1)
	}
	defer db.Close()

	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return fmt.Errorf("no file bucket found in %s", *dbpath)
		}
		err := b.ForEach(func(k, v []byte) error {
			path := filepath.Join(*outdir, string(k))
			// Make directory tree for this file
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			w, err := os.Create(path)
			if err != nil {
				return err
			}
			defer w.Close()
			zr, err := gzip.NewReader(bytes.NewReader(v))
			if err != nil {
				return err
			}
			defer zr.Close()
			if _, err := io.Copy(w, zr); err != nil {
				return err
			}
			color.Green("%s -> %s", string(k), path)
			return nil

		})
		return err
	})
	if err != nil {
		color.Red("error restoring files: %v", err)
		os.Exit(1)
	}
	color.Cyan("\nDone :)")
}
