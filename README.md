<div align="center">
    <img src="logo.png" alt="DaBaDee" width="450"/>
    <p>DaBaDee is a simple deduplication tool/storage for files. It uses SHA256* to
hash the files and store them in the storage, replacing the original path with
a hardlink to the storage location.</p>
    <sub>* SHA256 is the default hashing algorithm, but it can be replaced with
    any other hashing algorithm that implements the `hash.Generator` interface.
    The highwayhash algorithm is also provided as an example.</sub>
</div>

## Usage

### CLI

**Show help**

```sh
dabadee --help

Usage:
  dabadee [command]

Available Commands:
  completion     Generate the autocompletion script for the specified shell
  cp             Copy a file and deduplicate it in storage
  dedup          Deduplicate files in a directory
  find-links     Find all hard links to the specified file
  help           Help about any command
  remove-orphans Remove all orphaned files from the storage
  rm             Remove a file and its link from storage

Flags:
  -h, --help   help for dabadee

Use "dabadee [command] --help" for more information about a command.
```

**Deduplicate a folder**

```sh
dabadee dedup /path/to/folder --storage /path/to/storage --workers 2
```

Where the `/path/to/folder` is the folder to deduplicate and `/path/to/storage`
is the location you want to store the deduplicated files. The `2` is the number
of workers to use to speed up the process.

> Do not delete the resulting storage folder, as it contains the original files.

**Deduplicate a folder and obtain the pairings of origins and hashes in storage**

```sh
dabadee dedup /path/to/folder --storage /path/to/storage --workers 2 --manifest-output /path/to/manifest.json
```

This will create a JSON file with the pairings of the original files and their
hashes in the storage, like this:

```json
{
    "/path/to/folder/file1": "1234...",
    "/path/to/folder/file2": "1234...",
    ...
}
```

the path must point to a file that does not exist, not a folder. The hash is
the same as the one used in the storage, so the same algorithm will be used.

**Deduplicate on copy**

```sh
dabadee cp /path/to/file /path/to/dest/file --storage /path/to/storage
```

This will copy the file to the destination and deduplicate it in the storage if
not already present.

**Deduplicate a folder on copy**

```sh
dabadee cp --append /path/to/folder /path/to/dest --storage /path/to/storage --workers 2
```

This will copy the folder to the destination and deduplicate it in the storage
if not already present. The `--append` flag indicates to copy the folder
contents inside the destination folder.

**Keep metadata**

```sh
dabadee dedup /path/to/folder --storage /path/to/storage --workers 2 --with-metadata
dabadee cp /path/to/file /path/to/dest/file --storage /path/to/storage --with-metadata
```

This will keep the original file metadata (uid, gid, permissions) when copying
the file to the storage.

**Global Storage vs Scoped Storage**

When using the CLI, the storage can be defined globally or scoped. The scoped
storage is defined with the `--storage` flag, this allows you to define a new
storage each time you run a command. The global storage is assumed to be in the
user's home directory, in the `.dabadee/Storage` folder if Dabadee is run as
user, or in the `/opt/dabadee/Storage` folder if Dabadee is run as root.

```sh
# This will use the global storage:
dabadee dedup /path/to/folder --workers 2

# This will use the scoped storage:
dabadee dedup /path/to/folder --workers 2 --storage /path/to/storage
```

This behavior is currently supported by the `cp` and `dedup` commands only.

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
    s := storage.NewStorage(storage.StorageOptions{
        Root: "/path/to/storage",
        WithMetadata: true,
    })
    h := hash.NewSHA256Generator()
    p := processor.NewDedupProcessor("/path/to/folder", "/path/to/dest", s, h, 2)

    d := dabadee.NewDaBaDee(p)
    err := d.Run()
    if err != nil {
        panic(err)
    }
}
```

If the destination folder does not exist, it will be created, if it is not defined
(empty string), no files will be copied, and only the original files will be
deduplicated.

Please note that DaBaDee is not atomic, so if the process is interrupted, the
storage may be left in an inconsistent state. It is recommended to use a
transictional folder instead of working with the original files directly, and
then swap the folders when the process is done.

## What's with the name?

The name comes from the song ["Blue (Da Ba Dee)" by Eiffel 65](https://www.youtube.com/watch?v=68ugkg9RePc).
There is no connection between the song and the project, it's just the song I
was listening to when I started the project. So... I'm blue, da ba dee da ba daa.

## What is left to do?

- [x] Add tests
- [x] Provide better logs and ask user input when needed
- [x] Add a way to remove files from the storage reflecting the changes in the
      original files
- [x] Provide an option to respect metadata (uid, gid, permissions)
- [x] Split cmd and lib in two different packages
- [x] Add a way deduplicate files and folders on copy
- [x] Add a global storage configuration if the storage is not provided