# contemper

A Go CLI that converts a contemper-ready OCI image into a bootable VM
disk bundle.

```console
$ contemper convert --target qemu ghcr.io/example/my-appliance:v3
$ contemper deploy --to local-qemu my-appliance-v3.aarch64/
```

This document covers the implemented contract: what `convert`
and `deploy` actually do, and what an image must provide.

## What contemper reads from your image

`convert` requires the config label `io.contemper.ready=true`. This is
checked before any layer is fetched, so an unready image fails cheaply.

Beyond that, contemper reads:

- `Config.Labels` — the readiness label above, plus (on a support image)
  `io.contemper.requires.files`.
- `Config.Volumes` — recorded in the bundle manifest as-is.
- `Config.ExposedPorts` and `Config.Healthcheck` — recorded as
  deployment hints, unused by disk assembly.

`Cmd`, `Entrypoint`, `User`, `WorkingDir`, `Env` and everything else are
ignored: your init system, not a container runtime, drives the VM.

## The fixed-path contract

Every image must provide these at conversion time (after merging in an
optional support image):

| Path | Meaning |
| --- | --- |
| `/boot/contemper/vmlinuz` | the kernel |
| `/boot/contemper/initrd` | the initrd (generic, not host-only) |
| `/boot/contemper/cmdline` | the kernel command line (non-empty after trimming whitespace) |
| `/sbin/init` | must exist |
| `/etc/os-release` | optional; if present, becomes the UKI's `.osrel` section |

Symlinks are followed when resolving these paths, but only inside the
merged filesystem view - never on the host - with a hop limit and
clamping at `/`.

## Source references

A plain reference (`ghcr.io/foo/bar:v1`) means a registry, resolved with
the default keychain. Three other forms are accepted:

- `oci-archive:<path>` - what `podman save --format oci-archive` writes.
- `oci:<path>` - an OCI image layout directory.
- `docker-archive:<path>` - what `docker save` / `podman save
  --format docker-archive` writes.

There is no daemon source; point `convert` at a saved archive instead.
`--arch amd64|arm64` overrides the default of the host architecture. If
the reference is a multi-platform index, the matching manifest is
selected before anything is fetched; a missing match is a hard error.

## Merge and validation

1. **Readiness check** against the config label, before any layer is
   pulled.
2. **Merge**: the source image's layers, then (if `--support <ref>` is
   given) a support image's layers, flattened with whiteouts and opaque
   directories applied - the same view a container runtime would see.
   A support image is a plain rootfs overlay for the MVP: no variants or
   branches yet (see Scope below). If its manifest carries an
   `io.contemper.requires.files` annotation (a comma-separated list of
   absolute paths), every listed path must exist in the merged view.
3. **Validate** the merge against the fixed-path contract above.
4. **Assemble** the target's disk output (see Targets).

## Targets

| Target | Canonical name | Output |
| --- | --- | --- |
| `qemu` | `qemu-qcow2` | qcow2, UEFI-bootable UKI; no support image |
| `incus` | `incus-qcow2` | qcow2, UEFI-bootable UKI |

Both spellings are accepted; the canonical name is what gets recorded in
the bundle manifest. `qemu` is the reference target: nothing is added to
your image, and the disk boots under any QEMU with UEFI firmware. `incus`
builds the same disk format, since Incus runs VMs on QEMU/OVMF; it differs
only in the support image layered on top (not yet published).

### The disk

GPT, 1 MiB-aligned:

- Partition 1: a 128 MiB FAT32 ESP holding the UKI at
  `EFI/BOOT/BOOTAA64.EFI` (arm64) or `EFI/BOOT/BOOTX64.EFI` (amd64) - the
  UEFI fallback path, so no NVRAM boot entry needs registering.
- Partition 2: the ext4 root, last on the disk (so growing it later is
  the ordinary `growpart` + `resize2fs` pair), labeled
  `contemper-root`. Authors write `root=LABEL=contemper-root` in their
  cmdline. Sized `max(1 GiB, 1.5 x content + 256 MiB)` by default,
  overridable with `--root-size` (e.g. `--root-size 4GiB`).

The root filesystem is never extracted to the host under its own file
names: contemper builds it with `mkfs.ext4` plus a generated `debugfs`
script, so ownership, device nodes and hardlinks survive without ever
needing root privileges or a case-sensitive-filesystem assumption on the
build host. `--keep-raw` keeps the intermediate `disk.raw` next to the
qcow2 output.

### The UKI

Built in pure Go: an embedded `systemd-stub` binary (see `NOTICE`) with
`.osrel`, `.cmdline`, `.initrd` and `.linux` PE sections appended, in
that order. The kernel is used as-is if it's already an MZ/PE image,
decompressed if gzip-compressed, or passed through with a warning
otherwise.

## The bundle

`convert -o <dir>` writes `<dir>/<repo-basename>-<tag>.<arch>/`,
containing `disk.qcow2` and `contemper.json`. The manifest records the
format version, contemper version, build timestamp, source and support
image references and digests, the resolved target name, architecture,
disk file/format/size/sha256, volumes, deployment hints, and whether the
build is reproducible (registry sources are; archive and layout sources,
which can't be re-fetched byte-for-byte from their ref alone, are not).

## Deploying locally

`deploy --to local-qemu <bundle-dir>` boots the bundle's disk under a
host-discovered `qemu-system-*`, with `snapshot=on` so the bundle stays
pristine. It looks for `qemu-system-aarch64`/`qemu-system-x86_64` and
UEFI firmware on PATH and in common install locations (Homebrew's keg,
Debian's `*-efi`/OVMF packages), printing an install hint if nothing is
found.

- `--serial-log <file>`: where the VM's serial console is written.
- `--expect <string>`: exit 0 as soon as this string appears on the
  serial console (and kill qemu); useful for CI boot tests.
- `--timeout <duration>`: how long to wait for `--expect` before failing
  with the log tail.

## Progress output

`convert` and `deploy` print an icon-and-two-column progress narration to
stderr as each stage runs (source resolution, readiness, layer merge,
validation, assembly, the bundle). stdout only ever gets the bundle
directory path (`convert`) or nothing (`deploy`), so scripts can still
capture it.

- `--progress auto|tty|plain` - `auto` (the default) shows a live spinner
  and elapsed timer on an interactive terminal, replaced by a line per
  stage when it completes; `plain` always appends one line per stage
  with no cursor movement (used automatically for pipes/CI, `NO_COLOR`,
  or `TERM=dumb`).
- `-v, --verbose` shows each host tool invocation (`mkfs.ext4`,
  `debugfs`, `qemu-img`, `qemu-system-*`) as a dimmed line; their own
  output is otherwise shown only if they fail.
- `-q, --quiet` suppresses all progress output.

## Host tools

contemper is a single static-ish Go binary plus four host tools it
discovers rather than bundles: `mkfs.ext4`, `debugfs`, `e2fsck` and
`qemu-img`/`qemu-system-*`. Nothing from a converted image is ever
executed on the host; these are the only subprocesses contemper runs,
always as argv arrays with their own stderr passed through.

## Testing

`make test` runs the unit tests. Tests that need host tools (e2fsprogs,
`qemu-img`) skip when those tools are missing.

`make e2e` (`hack/e2e.sh`) is the end-to-end boot test. It builds
`examples/alpine` for the host architecture with podman or docker,
converts it with `--target qemu`, and boots it with
`deploy --to local-qemu`. It passes once `contemper-boot-ok` appears on the
serial console. It needs the host tools above plus `qemu-system-<arch>`
and UEFI firmware. CI runs it on x86-64 Linux.

## Scope

This is the MVP: `convert`, `deploy --to local-qemu`, two targets
(`qemu-qcow2`, `incus-qcow2`), and a plain-rootfs support image. Not yet implemented:
support-image variants/branches, `build`/`publish` adapters, an
`incus`-native deploy target, a project config file, multi-arch source
fan-out, provenance referrers, and xattrs/file-capability preservation
in the rootfs.

## License

Apache License 2.0, see `LICENSE`. The embedded systemd-stub binaries are
LGPL-2.1-or-later; see `NOTICE`.
