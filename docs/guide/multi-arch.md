# Multi-architecture

**contemper's own step needs no emulation.** There is no guest code to
run, so producing an arm64 disk on an x86-64 host is not a special case.

Building the image in the first place may well need emulation, since
`RUN` steps do execute guest code. That's handled by `docker buildx` or
native runners, at a stage the container ecosystem already solves well.
The point isn't that emulation disappears; it's that it stays confined
there instead of reappearing at conversion time.

Architecture handling uses standard OCI mechanisms. Source images and
support images alike resolve from image indexes, so no `-amd64`/`-arm64`
tag conventions are needed, and mismatches fail at manifest level rather
than at boot. Only the assembler is architecture-aware, and only for the
UEFI boot path and the EFI stub; the kernel and initrd are already in
your image.

## Choosing the architecture

`convert` builds for the host architecture by default. `--arch amd64` or
`--arch arm64` picks another. If the source is a multi-platform index,
the matching manifest is selected before anything is fetched, and a
missing match is an error.

!!! warning "Planned"
    Whether contemper should build every architecture in an index in one
    run is an [open question](../design/open-questions.md). For now, run
    `convert` once per architecture.
