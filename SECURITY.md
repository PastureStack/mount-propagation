# Security Boundary

`mount-propagation` is a host-administrative utility. Real operation can enter
another mount namespace and invoke `mount(2)` with `MS_SHARED | MS_REC`.

The legacy deployment model grants the helper:

- a privileged container boundary or equivalent `CAP_SYS_ADMIN` access;
- host PID visibility;
- a bind mount of the host root at `/host`;
- access to Docker/runc runtime state; and
- the `nsenter` executable.

These permissions are not a sandbox. A compromised binary or incorrect mount
target can affect the host. Do not expose the helper to untrusted workloads or
allow untrusted users to control its arguments.

Unprivileged unit tests intentionally stop at parser and temporary-file
boundaries. The real mount namespace POC must run in a disposable VM with a
snapshot or other reliable rollback mechanism. Never point the POC at a
production mount, a workstation filesystem, or a shared storage path.

## Disposable VM POC

Run the following only inside a disposable Linux VM. The procedure creates one
self-bind mount under `/mnt`, changes its propagation, verifies it, and then
unmounts it.

```sh
set -eu

go test -race -cover ./...
./scripts/build

docker build -t pasturestack/mount-propagation:poc -f - . <<'EOF'
FROM ubuntu:26.04
RUN apt-get update \
    && apt-get install -y --no-install-recommends util-linux \
    && rm -rf /var/lib/apt/lists/*
COPY bin/mount-propagation /usr/bin/mount-propagation
COPY bin/share-mnt /usr/bin/share-mnt
ENTRYPOINT ["/usr/bin/mount-propagation"]
EOF

docker run --rm \
  --entrypoint /usr/bin/mount-propagation \
  pasturestack/mount-propagation:poc --version
docker run --rm \
  --entrypoint /usr/bin/share-mnt \
  pasturestack/mount-propagation:poc --version

test "$(findmnt -no PROPAGATION /)" != "shared"
sudo mkdir -p /mnt/pasturestack-mount-propagation-poc
sudo mount --bind \
  /mnt/pasturestack-mount-propagation-poc \
  /mnt/pasturestack-mount-propagation-poc

docker run --rm \
  --privileged \
  --network=host \
  --pid=host \
  --cgroupns=host \
  --volume /:/host \
  pasturestack/mount-propagation:poc \
  /mnt/pasturestack-mount-propagation-poc -- norun

findmnt -no TARGET,PROPAGATION /mnt/pasturestack-mount-propagation-poc
sudo umount /mnt/pasturestack-mount-propagation-poc
sudo rmdir /mnt/pasturestack-mount-propagation-poc
```

The expected propagation value is `shared`. If container discovery or
`state.json` lookup fails, preserve the VM and capture `/proc/self/cgroup`, the
Docker version, and the runtime-state directory layout before changing code.
