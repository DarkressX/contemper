# Comparison

| Tool | Authoring input | Runs code to build | Scope |
| --- | --- | --- | --- |
| **contemper** | any OCI image | no | image → bootable disk |
| [bootc](https://github.com/bootc-dev/bootc) | `Containerfile` on a bootc base | yes (ostree) | image → disk **and in-place updates** |
| [Packer](https://www.packer.io/) | provisioner scripts | yes (boots an instance) | broad; many clouds, Windows too |
| [mkosi](https://github.com/systemd/mkosi) | declarative config and packages | yes (distro installers) | OS images, systemd-centric |
| [distrobuilder](https://github.com/lxc/distrobuilder) | YAML definitions | yes | Incus/LXC images |
| [virt-builder](https://libguestfs.org/virt-builder.1.html) | existing cloud image | yes | customizing existing images |
| [KubeVirt containerDisk](https://kubevirt.io/) | pre-built disk image | no | transport only, not authoring |

## bootc

The closest comparison is **bootc**, which validates the same thesis,
that container-authoring ergonomics suit OS images, with real vendor
backing. It differs in three ways that matter:

- **Base images.** bootc's base images are prescriptive: you start from
  one built for bootc. contemper accepts any base you install a kernel
  and init system into, Alpine and OpenRC included.
- **Scope.** Transactional in-place updates with `bootc switch` are
  bootc's actual point. contemper stops at producing a disk and takes no
  position on how a running VM is updated.
- **Semantics.** bootc's ostree-based conversion gives
  atomically-upgraded-host semantics: `/usr` read-only, `/etc` and `/var`
  merged forward. contemper gives container-like ones: a writable root
  that is kept across reboots and replaced on redeploy, with persistent
  data on volumes.

## The others

- **Packer and virt-builder** start from an existing image that already
  has a kernel installed, and customize it by running code, either by
  booting an instance or through libguestfs. contemper builds the OS
  from your container build instead and runs nothing at conversion time.
- **mkosi and distrobuilder** also build an OS from packages, as contemper
  does, but through their own configuration formats and by running
  distro installers. contemper's input is whatever your container tooling
  already produces.
- **KubeVirt containerDisk** wraps a finished disk image in an OCI image
  as an opaque payload. It solves transport, not authoring.
