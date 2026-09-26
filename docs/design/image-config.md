# What is read from the image config

contemper consumes OCI image configs, not build files, so the question
is not which Containerfile instructions are "supported" but which config
fields are read. Three groups fall out:

- Most instructions (`RUN`, `COPY`, `ADD`, `ARG`, `FROM`, `SHELL`) leave
  no config trace at all. They aren't ignored so much as invisible: they
  produce the filesystem, which is the actual input.
- The platform fields (`architecture`, `os`, `variant`) are consumed by
  multi-architecture resolution.
- The rest is the real decision surface. `Labels` and `Volumes` are
  consumed; `ExposedPorts` and `Healthcheck` are recorded as deployment
  hints; `Entrypoint`, `Cmd`, `User`, `WorkingDir`, `Env`, `StopSignal`
  and `OnBuild` are ignored.

## Ignored, never an error

The instinct to reject what isn't supported is reasonable, and wrong
here. These fields are *inherited*. `FROM alpine` yields
`CMD ["/bin/sh"]`; almost any non-scratch base sets `User`, `Env` or
`ExposedPorts`. Failing a build over a set `Cmd` would reject images
whose authors never wrote the value and cannot easily see it.

contemper could in principle tell author-set from inherited values by
reading the config's `history`, but that means parsing Docker's
human-readable `created_by` strings, which nothing specifies and no
implementation guarantees. Not a foundation to build validation on.

## Runtime fields belong to support images

`Cmd`, `Entrypoint`, `User`, `WorkingDir` and `Env` describe how a
container runtime would start a process, a role filled in a VM by the
init system. Translating them would mean generating service wiring
specific to whichever init system is present, which is both bespoke
boot-time logic and precisely the job support images exist to do. A
target that wants `Cmd` turned into a service unit, or `Env` materialized
where its init system will find it, can do that in its support image.
contemper stays out of it.

`Env` is the field most likely to disappoint quietly. An author writes
`ENV DATABASE_URL=...`, and in a container the runtime injects it into
PID 1's environment; in a VM nothing does, so it evaporates.
Materializing it to `/etc/environment` was considered and rejected: it is
contemper writing into the guest, and `/etc/environment` isn't read
uniformly anyway (PAM reads it, OpenRC services largely don't), so the
"just works" behavior it promises would not hold.

`ExposedPorts` and `Healthcheck` are ignored for a different reason: they
are meaningful, just not to disk assembly. A VM has its own address and
there is no port-publishing layer to configure. Their natural consumer is
deployment (a security group, an Incus proxy device, a firewall rule, a
provider health check), so they are recorded in the bundle manifest as
hints.
