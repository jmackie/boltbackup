package main

import (
	"bytes"
	"compress/gzip"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

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

		nworkers = flags.Int("nworkers", 20,
			"max number of active goroutines/open files")
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

	type File struct {
		Path  string
		Bytes []byte
	}
	files := make(chan *File)

	go func() {
		defer close(files)
		db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket(bucketName)
			if b == nil {
				color.Red("no file bucket found in %s", *dbpath)
				return nil // nothing else will happen
			}
			err := b.ForEach(func(k, v []byte) error {
				f := &File{Path: string(k)}
				zr, err := gzip.NewReader(bytes.NewReader(v))
				if err != nil {
					return err
				}
				defer zr.Close()
				b, err := ioutil.ReadAll(zr)
				if err != nil {
					return err
				}
				f.Bytes = b
				files <- f
				return nil

			})
			if err != nil {
				color.Red("error iterating over files: %v", err)
			}
			return nil
		})
	}()

	// Concurrency bookkeeping
	wg := &sync.WaitGroup{}
	workers := make(chan struct{}, *nworkers) // limits the number of open files

	for f := range files {
		wg.Add(1)
		go func(f *File) {
			defer wg.Done()

			workers <- struct{}{}        // join
			defer func() { <-workers }() // leave

			fullpath := filepath.Join(*outdir, f.Path)
			// Make directory tree for this file
			if err := os.MkdirAll(filepath.Dir(fullpath), 0755); err != nil {
				color.Red("error creating directory tree for %s: %v", fullpath, err)
				return
			}
			w, err := os.Create(fullpath)
			if err != nil {
				color.Red("error creating file %s: %v", fullpath, err)
				return
			}
			defer w.Close()
			if _, err := w.Write(f.Bytes); err != nil {
				color.Red("error writing file %s: %v", fullpath, err)
				return
			}
			color.Green("%s -> %s", f.Path, fullpath)
		}(f)
	}
	wg.Wait()
	color.Cyan("\nDone :)")
}
