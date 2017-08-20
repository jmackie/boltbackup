package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"flag"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/fatih/color"
)

func backup(arguments []string) {
	var (
		// Flags for this subcommand
		flags = flag.NewFlagSet("backup", flag.ExitOnError)

		backupfile = flags.String("f", "~/.backup",
			"backupfile: files to be backed up, specified as patterns")

		dbpath = flags.String("db", "",
			"boltdb file; created if it doesn't already exist")

		nworkers = flags.Int("nworkers", 20,
			"max number of active goroutines/open files")

		compression = flags.Int("compress", 9,
			"gzip compression level (0 <= x < 10)")

		maxage = flags.Int("maxage", 1,
			"how old does a file need to be in order to be updated")
	)
	if err := flags.Parse(arguments); err != nil {
		color.Red("error parsing flags: %v", err)
		os.Exit(1)
	}

	// Flag checking
	if *backupfile == "~/.backup" {
		*backupfile = filepath.Join(homeDir(), ".backup")
	}
	if _, err := os.Stat(*backupfile); os.IsNotExist(err) {
		color.Red("backupfile not found: %q", backupfile)
		os.Exit(1)
	}
	if *dbpath == "" {
		color.Red("-db missing: need a boltdb path")
		os.Exit(1)
	}
	if *compression < 0 || *compression > 9 {
		color.Red("invalid gzip compression: %d", *compression)
		os.Exit(1)
	}

	// Open the database
	db, err := bolt.Open(*dbpath, 0600, nil)
	if err != nil {
		color.Red("error opening boltdb: %v", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create the bucket (just once!)
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	})
	if err != nil {
		color.Red("error initialising %q bucket: %v", bucketName, err)
		os.Exit(1)
	}

	// Concurrency bookkeeping
	wg := &sync.WaitGroup{}
	workers := make(chan struct{}, *nworkers) // limits the number of open files

	paths, err := parseBackupfile(*backupfile)
	if err != nil {
		color.Red("error parsing backupfile %q: %v", *backupfile, err)
		os.Exit(1)
	}

	// Kick off the goroutines
	for _, path := range paths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			workers <- struct{}{}        // join
			defer func() { <-workers }() // leave

			k := []byte(path)

			r, err := os.Open(path)
			if err != nil {
				color.Red("%s: %v", path, err)
				return
			}
			defer r.Close()

			// Check modification times
			stale := true

			info, err := r.Stat()
			if err != nil {
				color.Red("%s: error getting file info: %v", path, err)
				return
			}

			err = db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket(bucketName)
				v := b.Get(k)
				if v == nil {
					return nil
				}
				zr, err := gzip.NewReader(bytes.NewReader(v))
				if err != nil {
					return err
				}
				// NOTE ModTime from the gzipped file header is floored to nearest second
				age := info.ModTime().Sub(zr.Header.ModTime)
				stale = age > (time.Duration(*maxage) * time.Second)
				return nil
			})
			if err != nil {
				color.Red("%s: error reading db file data: %v", path, err)
				return
			}

			// If the db file is up to date, stop here
			if !stale {
				color.Yellow("%s: up to date", path)
				return
			}
			// Write the updated file contents to the boltdb
			var buf bytes.Buffer
			zw, err := gzip.NewWriterLevel(&buf, *compression)
			if err != nil {
				color.Red("%s: error creating gzip writer (level %d): %v", path, *compression, err)
			}
			zw.Name = path
			zw.ModTime = info.ModTime() // important!

			if _, err := io.Copy(zw, r); err != nil {
				color.Red("%s: error copying file contents: %v", path, err)
				return
			}
			// Always flush
			if err := zw.Close(); err != nil {
				color.Red("%s: error flushing gzip writer: %v", path, err)
				return
			}
			err = db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket(bucketName)
				return b.Put(k, buf.Bytes())
			})
			if err != nil {
				color.Red("%s: error putting file in boltdb: %v", path, err)
				return
			}
			color.Green("%s: updated", path)
		}(path)
	}
	wg.Wait()
	color.Cyan("\nDone :)")
}

// The backup subcommand takes a "-f" flag (default: "~/.backup"), which is the path to a plain-text
// file specifying files to be added to the database. This plain-text file is intended to be .gitignore-like.
// This function parses that file (passed by path).
//
// Notes on file syntax:
//     + Lines beginning with '#' are skipped
//     + Lines beginning with '!' specify files to be *ignored*
//     + Lines are prefixed with the dirname of the file itself
//
func parseBackupfile(path string) ([]string, error) {
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	r, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	// Scanning lines
	scanner := bufio.NewScanner(r)

	// Use a map so that we can easily remove entries.
	files := make(map[string]struct{})

	// Lines are prefixed with the file dirname.
	dir := filepath.Dir(path)
	if dir[len(dir)-1] != os.PathSeparator {
		// "The returned path does not end in a separator unless it is the root directory."
		dir += string(os.PathSeparator)
	}

LineLoop:
	for i := 0; scanner.Scan(); i++ {
		line := scanner.Text()

		var bang bool // is this an "exclusions" line?
		switch line[0] {
		case '#':
			// Skip
			continue LineLoop
		case '!':
			// Matches should be IGNORED
			bang = true
			line = line[1:] // trim
		}
		pattern := dir + line
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		all, err := expandMatches(matches)
		if err != nil {
			return nil, err
		}
		for _, path := range all {
			if bang {
				delete(files, path)
			} else {
				files[path] = struct{}{}
			}
		}
	}

	// Pull out the keys
	result := []string{}
	for k := range files {
		result = append(result, k)
	}
	return result, nil
}

// Explodes directory paths into file leaves, and returns actual file paths untouched.
func expandMatches(matches []string) ([]string, error) {
	leaves := []string{}
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		switch {
		case info.IsDir():
			// path is a directory, so walk it
			// and pull out all files below.
			err := filepath.Walk(path, justFiles(&leaves)) // NOTE modifies leaves inplace
			if err != nil {
				return nil, err
			}
		default:
			// path is a plain file, nothing todo.
			leaves = append(leaves, path)
		}
	}
	return leaves, nil
}
