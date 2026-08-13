<div align="center">
    <img src="logo.png" alt="DaBaDee" width="450"/>
    <p>DaBaDee is a content-addressed file store with whole-file and chunk deduplication.</p>
</div>

DaBaDee stores one object for identical files and replaces destinations with hardlinks. Version 2 can also reuse unchanged ranges between similar files on filesystems that support reflink range cloning. The store remains useful as a standalone Go library and does not depend on cpak.

## Install

```sh
go get github.com/mirkobrombin/dabadee/v2
```

The `v2` suffix is required by Go modules. Version 1 remains available at `github.com/mirkobrombin/dabadee`.

## Library

```go
package main

import (
    "bytes"
    "context"
    "log"

    "github.com/mirkobrombin/dabadee/v2/pkg/store"
)

func main() {
    storage, err := store.Open(store.Options{
        Root:             "/path/to/storage",
        PreserveMetadata: true,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer storage.Close()

    _, err = storage.Import(
        context.Background(),
        "/path/to/destination",
        bytes.NewReader([]byte("content")),
        store.ImportOptions{},
    )
    if err != nil {
        log.Fatal(err)
    }
}
```

`Import` accepts an `io.Reader`, so callers can hash, decompress and store data without first writing another complete copy to disk. `ImportFile` imports an existing file and `DeduplicateTree` replaces regular files in a directory in place. `Contains` checks whether a known digest is reusable without creating a destination, while `Reuse` links it without asking the caller to supply its bytes again.

Call `GC` after deleting destinations. An object is removed only when no hardlink outside the store still references it. `CollectGarbage(ctx, false)` reports current garbage without changing the store. `PlanGC` can also include paths that the caller intends to remove. `Close` writes the index atomically and releases the process lock.

## Deduplication

Every imported file gets a SHA-256 content digest. When metadata preservation is enabled, its mode, UID and GID are part of the object key. Equal objects share one inode through hardlinks.

The input is also split at content-defined, 4 KiB-aligned boundaries. On a filesystem with reflink range support, unchanged chunks are cloned into the new object and share physical extents. A modified file can therefore reuse most of an existing object without storing a second full copy.

DaBaDee probes this capability when the store opens. Filesystems without reflink support keep whole-file deduplication and write similar files normally. It does not keep a separate chunk payload store, because that would duplicate data on filesystems that cannot share extents.

```go
capabilities := storage.Capabilities()
if capabilities.Reflink {
    // Similar files can reuse physical extents.
}
```

## Recovery

Imports are written in the v2 staging directory and published only after the content and metadata are complete. A later `Open` removes staging files left by an interrupted process. The object index is replaced atomically after its new contents have reached disk.

See [Migration from v1](docs/migration-v2.md) before changing an existing integration.

## CLI

The existing `cp`, `dedup`, `find-links`, `rm` and `rm-orphans` commands remain available for v1 stores. This keeps scripts and existing storage layouts working while applications move to the v2 library API. New integrations should use `pkg/store`.

## Name

The name comes from [Blue (Da Ba Dee) by Eiffel 65](https://www.youtube.com/watch?v=68ugkg9RePc). It was the song playing when the project started.
