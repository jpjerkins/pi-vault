# ADR-001: Synchronous Audit Log Writes

**Date:** 2026-03-20
**Status:** Accepted

## Context

`internal/crypto.go:AuditLog()` opens, writes one JSON line, and closes
`/mnt/data/vault-t2/.audit.log` on every call. In the FUSE daemon, this is
called on every secret read — including audit entries for `fuse-read`,
`fuse-denied`, `fuse-mount`, and `fuse-unmount`.

This means every secret read through the FUSE mount incurs three extra
syscalls: `open`, `write`, `close`.

## Decision

Keep synchronous writes. Do not buffer or batch audit log entries.

## Reasoning

**Usage profile makes the overhead negligible.** This vault serves a home
Raspberry Pi with a handful of Docker containers. Concurrent secret reads are
rare; sustained high-throughput reads do not occur. Three syscalls per read
is not a bottleneck at this scale.

**Synchronous writes are safer.** A buffered or async approach risks losing
audit entries if the daemon crashes or is killed mid-buffer. For a security
audit log, losing entries on crash is a worse outcome than slightly higher
per-read latency.

**`O_APPEND` is atomic for small writes.** A single `write()` of a JSON line
(< 512 bytes) to an `O_APPEND` file is atomic on Linux. This gives correct
behavior under the multi-threaded FUSE server without needing a mutex around
the log file.

## Consequences

If the vault is ever deployed in a higher-throughput context (e.g. many
containers reading secrets in tight loops), this decision should be
revisited. A background goroutine draining a buffered channel would be the
natural upgrade path, with a flush on `SIGTERM`.
