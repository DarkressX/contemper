# Support images

A support image carries what a target needs that your image shouldn't
have to know about: an agent binary, service definitions, boot
configuration. It is an ordinary OCI image.

**In its simplest form it is just a root filesystem.** Its layers merge
on top of your image, with no annotations and no conditions. Most
support images need no more than this.

```console
$ contemper convert --target qemu --support ghcr.io/example/extras:v1 \
    oci-archive:my-appliance.tar
```

**Support images drop in files; they never install anything.** A support
image contributes config files and static binaries, and nothing else.
Its layers are merged, never executed, so it cannot install something it
depends on. If a target needs cloud-init, your image has to already have
cloud-init; the support image supplies the configuration, not the
software.

## Requirements

A support image can declare paths your image must provide, as an
annotation on its manifest:

```text
io.contemper.requires.files=/usr/bin/cloud-init,/sbin/openrc-init
```

Every listed path must exist in the merged filesystem, or the conversion
fails naming the missing ones. This is how a support image asks for
something it cannot install itself.

## Variants

!!! warning "Planned"
    Variants are designed but not implemented yet. Today a support image
    is merged as a plain root filesystem, plus the requirements check
    above.

A support image will be able to declare **variants** of itself, each
selected by file-existence conditions, grouped into independent
**branches**:

- `init-system` picks the agent's service definition, an OpenRC init
  script or a systemd unit, by which init binary is actually present.
- `first-boot` picks configuration for whichever day-1 provisioning tool
  is installed.

The branches resolve independently, so any init system pairs with any
first-boot mechanism without a variant per combination. Architecture is
not a branch: support images resolve per architecture through ordinary
multi-arch image indexes.

Selection is deterministic: exactly one match per branch, or a declared
default. Zero matches or several both fail the build, naming the branch
and the paths checked. Candidates are evaluated by manifest first, so
losing variants are never downloaded.

Resolution is one level deep. Only the target's own support image is
examined for annotations; an image pulled in because it won a branch is
merged as-is. That rules out cycles and unbounded resolution chains by
construction.

The full mechanics, and why they look the way they do, are in
[Design: support image resolution](../design/support-images.md).
