# Migration from v1

DaBaDee v2 uses the Go module path `github.com/mirkobrombin/dabadee/v2` and adds the `pkg/store` API. Version 1 remains available and existing programs can keep using it without changing their stored files.

## Storage layout

Version 1 stores objects directly under the configured root and writes its options to `.dabadee`. Version 2 keeps its index, objects and staging data under `.dabadee-v2` in the same root.

Opening a v1 root does not move or copy its payload. When v2 imports content that already exists in the v1 store, it verifies the old object and adopts it with a hardlink. Both layouts can therefore coexist while the application migrates. The v1 object remains valid for old clients.

```go
storage, err := store.Open(store.Options{
    Root:             existingRoot,
    PreserveMetadata: true,
})
```

Use the same `PreserveMetadata` value as the v1 `WithMetadata` option. The v2 object key uses the same content and metadata hashes, so matching v1 objects can be adopted.

## Application changes

Replace the module imports and construct a v2 store:

```go
import "github.com/mirkobrombin/dabadee/v2/pkg/store"

storage, err := store.Open(store.Options{Root: root})
if err != nil {
    return err
}
defer storage.Close()
```

Use `Import` for streams, `ImportFile` for an existing file, and `DeduplicateTree` for an in-place directory pass. Call `GC` only after the application has removed references it no longer needs.

## Interrupted upgrades

There is no full-store conversion step. Adoption happens one object at a time, and a crash cannot invalidate the v1 object. On the next open, DaBaDee removes incomplete v2 staging files and resumes with the objects that were already published.

Do not delete the v1 objects until every v1 client has been retired. Their hardlinks do not consume another copy of the file data.

## Rollback

Version 1 ignores `.dabadee-v2`, while version 2 leaves `.dabadee` and all v1 object paths unchanged. Rolling the application back to v1 therefore does not require rewriting the store. Objects first introduced only through v2 are not visible to v1 clients unless the application exports them into the v1 layout.
