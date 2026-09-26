# Source references

`convert` takes one source reference: the image to convert. `--support`
takes another, in the same forms.

| Form | Meaning |
| --- | --- |
| `ghcr.io/example/app:v1` | a registry reference, pulled with your registry credentials (the same keychain docker and podman use) |
| `oci-archive:<path>` | an OCI archive, as written by `podman save --format oci-archive` |
| `oci:<path>` | an OCI image layout directory |
| `docker-archive:<path>` | a docker archive, as written by `docker save` or `podman save --format docker-archive` |

There is no container-daemon source. Save the image to an archive and
point `convert` at that instead. Nothing needs to be pushed to a
registry to iterate locally.

## Platform selection

`convert` builds for the host architecture unless `--arch amd64|arm64`
says otherwise. When a reference names a multi-platform index, the
manifest for that platform is selected before any layer is fetched; if
there is none, the conversion fails at that point.

## Recorded references

The bundle manifest records each reference in the same form, with local
paths cleaned up (`a/../b.tar` becomes `b.tar`), and the digest of the
image that was actually used. Bundles from local archives and layouts
are marked as not reproducible, since a path can't be fetched again the
way a registry digest can.
