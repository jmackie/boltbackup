# boltbackup: A backup utility built on [boltdb][boltdb_gh]

Yet another file backup tool, designed around my own personal requirements:

- Backup targets should be easily expressed in a single file, like a `.gitignore`. I'm choosing to call it a [Backupfile](#the-backupfile)
- Backup config should live with the rest of my system config, i.e. as a dotfile in `$HOME`
- The backup output should be a single, compact file
- The backup process should be fairly speedy so that I do it regularly
- Permissions and other file metadata don't matter

The command takes a list of file paths/patterns (via a [Backupfile](#the-backupfile)) and writes the gzipped contents of those files to a [Bolt database][boltdb_gh]. Bolt is a key/value store that works via a single local file, rather than a full database server. 

## Installation

```
go get github.com/jmackie4/boltbackup
```

## Usage

```
boltbackup is file backup utility, built on boltdb

Usage:

	boltbackup command [arguments]

The commands are:

	backup      put a bunch of files into a boltdb file
	restore     get files out of a db
	ls          list files that exist in a db


See also:
	boltdb          https://github.com/boltdb/bolt
	pattern syntax  https://golang.org/pkg/path/filepath/#Match

```

## The Backupfile

An example might be:

```
# ~/.backup
.*
stuff/
code/*.go
more_stuff/
!more_stuff/boring/
```

This file and the patterns it contains will resolve to a list of files that should be backed up. You would pass it to the `boltbackup` command like so:

```
boltbackup backup -f ~/.backup ...
```

*NB: `~/.backup` is the default value for the `-f` flag*. 

The resulting file list is much as you'd expect if you're used to working with `.gitignore`, `.dockerignore` and friends..

- All files beginning with a period
- All files in the `code` directory ending with `.go`
- All files in the `more_stuff` directory, except files in the `boring` subdirectory.

It is important to note that the dirname of the Backupfile is prepended to every pattern in that file. So important that I'm going to write it again with emphasis. 

> The dirname of Backupfile is prepended to every pattern in that file

This might smell a bit, but it fits neatly with how I intend to use the tool. If I can find a compelling reason to change this behaviour I will. 

## TODO

Quite a lot. 


[boltdb_gh]: https://github.com/boltdb/bolt

