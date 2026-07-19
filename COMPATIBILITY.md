# Compatibility Contract

This document separates PastureStack product naming from interfaces that must
remain compatible with existing legacy 1.6 agent deployments.

## Preferred PastureStack Interfaces

- Repository and module: `github.com/PastureStack/mount-propagation`.
- Primary executable: `mount-propagation`.
- Primary artifact: `mount-propagation-<architecture>.tar.gz`.

## Preserved Downstream Contracts

The following legacy interfaces are intentionally retained:

- The `share-mnt` executable alias.
- The `--stage2` internal command-line option.
- Existing positional mount targets and the `--` argument separator.
- Detection of cgroup-v1 `devices:/docker/<id>` entries.
- Detection of systemd `docker-<id>.scope` entries.
- Docker/runc runtime-state paths listed in `statePaths`.
- The runtime `state.json` fields `id`, `init_process_pid`, and
  `config.rootfs`.

The historical agent integration copied a binary named
`share-mnt` into `/usr/bin/share-mnt` and invoked it with mount targets similar
to:

```text
/usr/bin/share-mnt /var/lib/pasturestack/volumes /var/lib/kubelet -- norun
```

The `share-mnt` alias, Docker/runc identifiers, and the known runtime paths are
compatibility strings. New PastureStack integrations must use the neutral
volume root shown above; historical paths are recorded only in the private
migration knowledge base.

## Test Boundary

Normal unit tests are unprivileged and use in-memory input plus `t.TempDir()`.
They do not inspect the host's real container state and do not call
`syscall.Mount` or `nsenter`.

The real namespace POC must run only in a disposable Linux VM. It must verify:

- both executable names;
- cgroup and runtime-state discovery for the VM's Docker version;
- `nsenter` resolution;
- the `--stage2` re-execution path;
- recursive shared propagation on a disposable bind mount; and
- cleanup of that mount before destroying the VM.

## Known Compatibility Risk

Modern Docker/containerd installations may store runtime state outside the
currently enumerated runc paths or use a different state schema. A successful
build and parser test do not prove namespace compatibility. The isolated VM
POC is a release gate.

## Localization

This is a headless Linux system utility with no runtime i18n framework. Public
project documentation is maintained in English; translated runtime UI is not
applicable.
