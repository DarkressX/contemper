# Root filesystem

**Decided: a flat, writable ext4 root.** The disk is an ESP followed by
an ext4 root partition, root last so growing it is the ordinary
`growpart` plus `resize2fs` operation every cloud image uses. No
squashfs, no overlayfs, no synthetic union: the kernel mounts the root
partition directly and boot is over.

## The lifecycle it serves

A VM reboot *is* a restart: the instance survives it, so its filesystem
state should too. Docker behaves the same way. A container's writable
layer belongs to the container rather than to the run, and is destroyed
by `docker rm`, not by `docker restart`. So the root filesystem is kept
across reboots and shutdowns, and reset only when the disk is replaced
on redeploy. See [The disk and VM lifecycle](../guide/disk.md).

## Why not squashfs plus an overlay

The alternative was a read-only squashfs root with a writable overlay
partition, unioned with overlayfs. Its appeal was matching container
mechanics: the same lower/upper split container storage drivers use,
which looked like a strong selling point for an audience that knows
containers. Three things decided against it:

- **The lifecycle doesn't need it.** Once reboot was defined as a restart
  rather than a recreate, the overlay's distinguishing behavior,
  resetting the root while keeping volumes, was no longer exercised in
  normal operation. Replacing the disk already provides it.
- **The container argument inverts.** Container storage drivers run
  overlayfs over ordinary directories on an ordinary filesystem; nobody's
  lower layer is a squashfs. A plain writable filesystem is *closer* to
  how containers actually work.
- **Persistence made its costs worse.** With the overlay persisting
  indefinitely, overlayfs whole-file copy-up accumulates over an
  instance's entire lifetime, against a fixed-size overlay partition.

Dropping it also removed a disproportionate amount of machinery: an
initrd that mounts the squashfs, assembles the overlay and switches root;
a partition-UUID override for growpart, since `/` would have no backing
block device; the `mksquashfs` dependency; and distros that ship squashfs
only in optional kernel module packages.

## What was given up

These would justify revisiting the decision:

- **Compression at rest**, roughly 2–3× on a typical root filesystem with
  zstd. Partly recoverable for qcow2 with `qemu-img convert -c`, though
  qcow2 compression degrades as clusters are rewritten.
- **A guaranteed-immutable base**, the conventional pairing for
  dm-verity and verified boot. dm-verity also works over a read-only
  ext4, so this is conventional rather than exclusive to squashfs.
- **A shareable read-only root** across many VMs booting the same image.
  The rejected layout wouldn't have realized this anyway, since the
  squashfs sat on the same disk as the overlay.

None is load-bearing for a first release, and all stay reachable: the
assembler is the only component that cares about the on-disk layout.

The decision that *is* load-bearing, independently of this one, is
keeping **persistent state on attached volumes rather than in the root
filesystem**. That is what makes the lifecycle work and replacing a VM
cheap.
