# Volumes and providers

## Volumes

Persistent volumes are identified by the orchestrator when they are
attached, for example through a device `serial=` field, not by scanning
filesystem labels across a fleet. First attachment formats and labels an
empty volume; later boots recognize the label. Because identity is owned
by the orchestrator rather than by a fleet-wide lookup, label collisions
between unrelated volumes are harmless.

The exact identifier mechanism is provider-specific. contemper assumes
*some* stable identifier exists but does not prescribe one.

How volume metadata travels with the image, and what acts on it, is the
largest open question; see [open questions](open-questions.md). The
starting point is `VOLUME` in the image, recorded in the bundle manifest.

## Providers

Two providers come first:

- **qemu** is the reference case: it proves the disk boots with nothing
  but a hypervisor, and doubles as the debugging baseline. Implemented
  as `deploy --to local-qemu`.
- **Incus** is the real integration case, exercising agent injection,
  configuration and volumes. **Planned.**

The Incus provider must assume a **remote** Incus server. Local paths are
not a safe assumption anywhere in the deploy path: the disk must be
transferable, and after import everything refers to the image by
fingerprint or alias rather than by file path.

## Driving provider tools

Providers are implemented by running their own CLI tools as
subprocesses, which keeps provider-specific knowledge in the tool that
owns it. The conventions:

- Parse only structured output (`--format json`, `--output=json`), never
  human-readable text.
- Run tools with argument arrays, never shell strings.
- Check the tool is present, and its version, before doing any work.
- Pass the tool's error output through verbatim on failure.
- Decide partial-failure and rollback behavior explicitly, since
  subprocesses give no transactions.
