# Rejected approaches

**A custom OCI artifact type for VM images.** Early designs explored an
`artifactType` for disk images, shipping container layers as qcow2
backing-file deltas. Rejected because container layers are *file-level*
diffs while qcow2 backing chains are *block-level*: writing a layer's
file changes into a real filesystem dirties scattered clusters through
journal and metadata updates, so delta size stops tracking what logically
changed. It also meant custom media types, backing-file path rewriting,
and a content-addressed local store to resolve chains, a lot of machinery
for a benefit plain OCI form provides anyway. If efficient incremental
distribution ever matters, content-defined chunking (casync/desync) or an
ostree-style object store are better answers, since they dedup by content
rather than by block position.

**A squashfs root with a writable overlay.** Rejected once the VM
lifecycle was defined. See [Root filesystem](root-filesystem.md) for the
full reasoning and what was given up.

**Executable plugin binaries for targets.** Per-target logic shipped as a
binary inside a target-specific builder image, in the style of Cloud
Native Buildpacks or Terraform providers, is **rejected, not deferred**.
A plugin delivered in an image and run during conversion is image code
executing on the build host, which is exactly what contemper never does.
Adopting it would give up the bounded trust surface and the
emulation-free cross-architecture story. A plugin mechanism using
host-installed binaries would not break that principle, but is not
planned either: targets change rarely enough that ordinary releases
suffice, and versioning a plugin boundary is real ongoing cost.

**Running build steps against the authored image.** A templated
Containerfile with `FROM <authored image>` and `RUN` steps would allow
package-manager-aware customization that file overlays can't do.
Rejected for the same reason as plugins: it needs a build engine at
conversion time, executes code against untrusted content, and brings
network and package-version nondeterminism into every build.

**A separate provider/format axis.** Modeling output format (qcow2, raw)
and provider (incus, ec2) as independent dimensions was collapsed into
one flat target namespace with an optional format suffix. The two aren't
independent: most combinations are meaningless, and a single identifier
is simpler to select and document.

**contemper defining a build file format.** A `Machinefile` or similar was
considered and dropped once it was clear contemper never parses a build
file at all. The build file belongs to whatever tool produced the image.

**Embedding a build mechanism.** Building the root filesystem from a
Containerfile inside contemper would duplicate Docker, BuildKit and
Buildah, and pull in runc, containerd and seccomp bindings. The premise
is reusing container build tooling, not replacing it.
