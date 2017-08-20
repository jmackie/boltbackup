package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/boltdb/bolt"
	"github.com/fatih/color"
)

func ls(arguments []string) {
	var (
		// Flags for this subcommand
		flags = flag.NewFlagSet("ls", flag.ExitOnError)

		dbpath = flags.String("db", "",
			"boltdb file")
	)
	if err := flags.Parse(arguments); err != nil {
		color.Red("error parsing flags: %v", err)
		os.Exit(1)
	}

	// Flag checking
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
			return nil
		}
		b.ForEach(func(k, v []byte) error {
			fmt.Println(string(k))
			return nil

		})
		return nil
	})
	if err != nil {
		color.Red("ls error: %v", err)
	}
}
