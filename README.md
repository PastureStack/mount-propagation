# PastureStack Mount Propagation

`mount-propagation` enters the mount namespace associated with a container and
marks selected mount points as recursively shared. It is maintained by the
PastureStack community for legacy host-agent compatibility.

## Independent Community Project

PastureStack is an independent community effort to preserve, audit, and modernize the Rancher 1.6 ecosystem. It is not affiliated with or endorsed by Rancher Labs or SUSE.

**Upstream:** [`rancher/share-mnt`](https://github.com/rancher/share-mnt). This GitHub fork retains the upstream Git history, authorship, dates, and license notices unchanged; PastureStack maintenance is consolidated into one commit after the preserved upstream boundary.

Third-party names and marks remain the property of their respective owners. See
[ORIGIN.md](ORIGIN.md) and
[COMPATIBILITY.md](COMPATIBILITY.md) for provenance and compatibility details.

## Names and Compatibility

The preferred executable is `mount-propagation`. Builds also produce a
`share-mnt` executable alias because existing agent images and startup scripts
use that exact path.

The primary package artifact is:

```text
dist/artifacts/mount-propagation-<version>-linux-<architecture>.tar.gz
```

It contains both:

```text
mount-propagation
share-mnt -> mount-propagation
```

The internal `--stage2` option is also retained because the first process uses
it when re-executing itself through `nsenter`.

## Build

Build on Linux:

```sh
./scripts/build
```

Package the primary artifact and compatibility alias:

```sh
VERSION_OVERRIDE=v1.0.11 SOURCE_DATE_EPOCH=0 ARCH=amd64 ./scripts/package
```

The `release.yml` workflow checks the `v1.0.11` raw Linux amd64 binary on
pull requests and `main`: Go 1.27.0 build identity, race tests, vet, source and
binary vulnerability/secret scans, a CycloneDX SBOM, and SHA-256 checksums.
It publishes nothing automatically. A maintainer may dispatch it from `main`
after the checks pass; the workflow then creates an annotated numeric tag and
attaches the verified raw binary, archive, SBOM, scan evidence, and checksums
to the GitHub Release. The published `v1.0.11` release includes the raw binary
and `SHA256SUMS`; verify the downloaded file before deploying it.

## Unprivileged Tests

The unit tests exercise CLI help, cgroup parsing, `/proc/<pid>/stat` parsing,
mountinfo parsing, and temporary runtime-state directories. They do not call
`mount(2)`, `nsenter`, or privileged container APIs:

```sh
go test -race -cover ./...
```

Safe CLI checks after building are:

```sh
./bin/mount-propagation --help
./bin/mount-propagation --version
./bin/share-mnt --version
```

## Runtime Safety

Real operation requires Linux, `nsenter`, host PID visibility, access to
Docker/runc runtime state, and permission to change mount propagation. Those
permissions are effectively host-administrative. Never run the real namespace
POC on a workstation or production host; use a disposable Linux VM and follow
[SECURITY.md](SECURITY.md).

This headless system utility has no graphical UI or runtime localization
framework. No artificial language selector is added by this migration POC.

## License

The original Apache License, Version 2.0 file is retained unchanged. Git
history remains the authoritative record of original authors and later
contributors.
