# DaBaDee

DaBaDee is a simple deduplication tool/storage for files. It uses SHA256 to
hash the files and store them in the storage, replacing the original path with
a hardlink to the storage location.

## Usage

### CLI

**Show help**

```sh
dabadee --help
```

**Deduplicate a folder**

```sh
dabadee dedup /path/to/folder /path/to/storage 2
```

Where the `/path/to/folder` is the folder to deduplicate and `/path/to/storage`
is the location you want to store the deduplicated files. The `2` is the number
of workers to use to speed up the process.

> Do not delete the resulting storage folder, as it contains the original files.

**Deduplicate on copy**

```sh
dabadee cp /path/to/file /path/to/dest/file /path/to/storage
```

This will copy the file to the destination and deduplicate it in the storage if
not already present.

**Keep metadata**

```sh
dabadee dedup /path/to/folder /path/to/storage 2 --with-metadata
dabadee cp /path/to/file /path/to/dest/file /path/to/storage --with-metadata
```

This will keep the original file metadata (uid, gid, permissions) when copying
the file to the storage.

### Library

```go
package main

import (
    "github.com/mirkobrombin/dabadee/pkg/dabadee"
    "github.com/mirkobrombin/dabadee/pkg/hash"
    "github.com/mirkobrombin/dabadee/pkg/processor"
    "github.com/mirkobrombin/dabadee/pkg/storage"
)

func main() {
    s := storage.NewStorage("/path/to/storage")
    h := hash.NewSHA256Generator()
    p := processor.NewDedupProcessor("/path/to/folder", s, h, 2, true)

    d := dabadee.NewDaBaDee(p)
    err := d.Run()
    if err != nil {
        panic(err)
    }
}
```

Please note that DaBaDee is not atomic, so if the process is interrupted, the
storage may be left in an inconsistent state. It is recommended to use a
transictional folder instead of working with the original files directly, and
then swap the folders when the process is done.

## What's with the name?

The name comes from the song ["Blue (Da Ba Dee)" by Eiffel 65](https://www.youtube.com/watch?v=68ugkg9RePc).
There is no connection between the song and the project, it's just the song I
was listening to when I started the project. So... I'm blue, da ba dee da ba daa.

## What is left to do?

- [ ] Add tests
- [ ] Add a logger
- [ ] Add a progress bar
- [ ] Make access to storage more robust, with a lock file
- [x] Provide an option to respect metadata (uid, gid, permissions)
- [x] Split cmd and lib in two different packages
