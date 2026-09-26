# Targets

A target names what `convert` builds: the platform, and implicitly the
output format.

| Target | Canonical name | Output |
| --- | --- | --- |
| `qemu` | `qemu-qcow2` | qcow2, UEFI-bootable UKI, no support image |
| `incus` | `incus-qcow2` | qcow2, UEFI-bootable UKI, plus the Incus support image |

`qemu` is the reference target. It adds nothing to your image, and the
disk boots under any QEMU with UEFI firmware and nothing else. It is the
debugging baseline, and what `deploy --to local-qemu` is for.

`incus` builds the same disk format, since Incus runs VMs on QEMU with
OVMF firmware. It differs only in the support image layered on top.

!!! warning "Planned"
    The Incus support image (agent, service definitions, boot
    configuration) is not published yet, so an `incus` build currently
    produces the same disk as `qemu`.

## Names and aliases

Where a platform needs more than one format, it's a suffix on the target
name, `incus-qcow2` vs. `incus-raw`, in the spirit of Packer's builder
naming. The unsuffixed name is an alias for that platform's default. One
flat namespace, rather than platform and format as independent axes,
avoids a large surface of mostly meaningless combinations.

Both spellings work. The suffixed one is canonical, and it's what the
progress output and the bundle manifest record: `--target incus` is
reported as `incus-qcow2`. A platform's default format won't change
under you, since that would silently alter what your pipeline produces,
so it's treated as a breaking change.

## Targets and providers

A *target* is what `convert` builds: which support image went in, and
which format came out. A *provider* is where a bundle goes afterwards:
`local-qemu`, an Incus server. They are separate on purpose. An
`incus-qcow2` bundle boots fine under `local-qemu`; the target records
what went *into* the disk, not where it may run.
