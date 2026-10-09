# FFP1 — FarsiForge Patch Format v1

FFP1 is the binary patch format used by the FarsiForge end-user patcher. It is a
generalization of the Machine Party "MPP1" format: instead of a single Godot
`.pck` base file, FFP1 supports **multiple target files** in one patch, each
rebuilt from a list of records (copy-from-base or embedded compressed payload).

## Encoding rules

- All integers are **little-endian** (the default of `System.IO.BinaryReader` /
  `BinaryWriter` and Go's `encoding/binary` `LittleEndian`).
- All strings are **UTF-8**, length-prefixed with a `u32` byte count.
- Hashes: MD5 = 16 bytes, SHA-256 = 32 bytes.

## Layout

```
Header:
  magic "FFP1"                 4 ASCII bytes
  u32  version = 1
  metadata: 8 strings, each (u32 len + UTF-8 bytes):
    patch_name
    game_name
    game_exe          (game-relative path to the game executable, e.g. "Game.exe")
    engine            (e.g. "Unity", "Godot", "Unreal")
    patch_version
    author
    description
    created_at        (ISO 8601, e.g. "2026-10-05T12:00:00Z")
  u32 target_count
  i64 blob_size        (total size of the DEFLATE blob at the end)

Targets (repeated target_count times):
  u32  path_len + path UTF-8     (game-RELATIVE target file path,
                                 e.g. "SupermarketTogether_Data/resources.assets")
  u8   mode                      (0 = rebuild from records, 1 = replace = single payload record)
  i64  final_size               (uncompressed size of the final target file)
  32 bytes sha256               (of the final target file)
  u32  record_count
  Records (repeated record_count times):
    u32  path_len + path UTF-8   (logical name, for logging)
    u8   src                     (0 = copy-from-base, 1 = embedded payload)
    if src == 0:
      u32  base_len + base_path UTF-8  (game-relative base file)
      i64  offset                (byte offset into the base file)
      i64  size                  (number of bytes to copy)
    if src == 1:
      i64  payload_ofs           (offset into the blob, blob-relative)
      i64  z_size                (compressed (DEFLATE) size)
      i64  raw_size              (uncompressed size)
    16 bytes md5                 (of the record's uncompressed content)

Blob: raw DEFLATE streams (System.IO.Compression.DeflateStream-compatible,
      Go compress/flate BestSpeed/Default). Concatenated, in any order; records
      reference them by payload_ofs.
```

## Semantics

### Apply (install)

For each target, the patcher writes `<target>.ffnew` by streaming its records
in order:

- `src == 0` (copy-from-base): open the base file at `base_path` (relative to
  the game folder), seek to `offset`, copy `size` bytes into `.ffnew`, and
  verify the MD5 of the copied bytes against the record's `md5`.
- `src == 1` (embedded payload): seek the patch blob at `blob_start +
  payload_ofs`, read `z_size` bytes, DEFLATE-inflate to `raw_size` bytes,
  verify MD5, write into `.ffnew`.

After all records are written, the patcher verifies the SHA-256 of the whole
`.ffnew` against the target's `sha256` and `final_size`. If it matches, the
original target file is renamed to `<target>.ffbak` and `.ffnew` is renamed to
the target path.

If any target fails mid-apply, already-swapped targets are restored from their
`.ffbak` and all `.ffnew` temps are deleted (rollback).

### Uninstall

For each target in the patch, if `<target>.ffbak` exists, the patched file is
deleted and `.ffbak` is renamed back to the target path. Uninstall needs only
the patch file (for the target list) and the game folder.

### Modes

- `mode == 0` (rebuild): the target is assembled from multiple records (mix of
  copy-from-base and embedded payloads). Used for in-place patching of large
  asset bundles where only part of the file changes.
- `mode == 1` (replace): the target is a single embedded payload record. Used
  for full-file replacement (simplest case). The record's `path` is typically
  the same as the target path.

## Compatibility

The format is byte-identical between the C# reader (`patcher/FarsiForgePatcher.cs`)
and the Go writer (`pkg/ffpatch/ffpatch.go`). Cross-validation
(`pkg/ffpatch/crossval_test.go`) builds the C# CLI, writes a patch with the Go
writer, applies it with the C# CLI, and verifies the result.