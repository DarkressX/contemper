# Principles

**As simple as possible, as complex as required.** When two designs both
meet the requirement, take the boring one. Complexity has to be earned
by a requirement that exists now, not by one that might appear later.
This is the tie-breaker when other principles don't settle a question,
and much of what follows is an instance of it: support-image resolution
stops at one level, predicates are file-existence checks rather than an
expression language, targets are one flat namespace, there is no plugin
system, and the root filesystem is a plain writable one rather than a
squashfs union. The principle forbids unearned *complexity*, not cheap
groundwork: target aliases were implemented while only one target
existed, because an alias is a table entry and retrofitting the naming
later would break scripts.

**Never execute anything from an image.** contemper copies files and
merges layers, and that is all it does to image content. Detection,
selection and validation are read-only inspection, and no binary from an
image is ever run *on the build host*, at any stage, for any reason.
This bounds the trust surface, removes any need for a container runtime
at conversion time, and makes cross-architecture builds unremarkable.
Two boundaries keep the principle precise. Helper binaries on the build
host are allowed: contemper runs tools it chooses (`mkfs.ext4`,
`qemu-img`, `incus`), which are host-native and trusted. And the
principle is about the build host, not the produced VM: the author's
kernel, init system and workload obviously run once the disk boots.

**Reuse existing container tooling.** No new build system and no new
manifest format: the filesystem is produced by whatever container build
tooling the author already has. Embedding a Containerfile-to-rootfs build
inside contemper was ruled out early. It would duplicate what Docker,
BuildKit and Buildah already do well, and pull in runc, containerd and
seccomp bindings, at odds with shipping a small static binary.

**No new artifact format.** Everything contemper reads is a normal,
OCI-compliant container image. Standard registries, tooling, signing and
scanning all apply unmodified. Registries are also the transport:
content addressing, layer dedup and the existing auth, signing and
scanning ecosystem come free.

**Fundamentals live at a fixed path.** Kernel and initrd are read from
one contemper-defined location for every target. Target-specific
requirements are declared as data by support images, but the baseline
is not negotiable per target. A single convention is what makes an
image portable across targets without rebuilding.

**Init-system agnostic.** Alpine and OpenRC are first-class, not an
afterthought on a systemd-shaped design. contemper only detects which
init system is present, so a support image can supply matching service
integration. This is why the support-image variant mechanism exists: a
design that assumed systemd would not need one.

**Replacement is the cheap path, not the mandated one.** contemper ships
no update mechanism and takes no position on how a deployed VM is
updated. What it provides is an output where persistent state lives on
attached volumes rather than in the root filesystem, so replacing a VM
wholesale is cheap and safe rather than a data-loss event. Authors who
prefer in-place updates can use whatever package manager their image
already carries.

**No opinion on image size.** What goes into the image is the author's
choice. Minimal images suit the design best, but smallness is not a
contemper constraint.

**No opinion on day 2.** How a booted system receives configuration,
grows its disk, or gets updated is a property of the provider and the
author's image. contemper produces a bootable starting point and stops.

**Provider-specific things stay provider-specific.** How a target
identifies attached volumes, for example by device serial, is left to
that target's conventions rather than prescribed centrally.

**No container runtime at conversion time.** Go, statically linked where
practical, preferring libraries over subprocesses, with a small set of
accepted host tools (currently the e2fsprogs family and `qemu-img`).
A conversion step that runs anywhere, with no runtime and no privileged
builder, is what lets it drop into an ordinary CI job next to the
container build before it.

**A defined interface for per-target logic.** Per-target code takes a
merged filesystem and produces a target output, rather than reaching
into contemper's internals. That keeps the shared pipeline shared
instead of accumulating per-target special cases, and lets an assembler
be tested without a registry or a real image.
