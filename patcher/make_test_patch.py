#!/usr/bin/env python3
"""make_test_patch.py — generate a test .ffpatch file (FFP1 format) from a
fake game layout. Pure stdlib (struct, zlib, hashlib).

Usage:
    python make_test_patch.py <game_dir> <patch_dir> <out.ffpatch>

Layout:
    game_dir/        — the "original" game files (the base). Must contain the
                       files referenced by copy-from-base records.
    patch_dir/       — the "patched" versions of files. Each file here becomes
                       a target with mode=1 (replace = single embedded payload
                       record). The file's relative path is the target path.
    out.ffpatch      — the produced FFP1 patch.

The script also adds one copy-from-base record (mode=0 target) if a file
`base_source.bin` exists in game_dir: it copies the first 64 bytes of that
file into a new target `rebuilt.bin` (mode=0, one copy-from-base record),
demonstrating the rebuild mode.

The produced patch is byte-compatible with the C# FarsiForgePatcherCli.exe
and the Go pkg/ffpatch writer.
"""

import sys
import os
import struct
import zlib
import hashlib
import datetime


def write_str(buf, s):
    b = s.encode("utf-8")
    buf += struct.pack("<I", len(b))
    buf += b


def deflate_raw(data):
    # zlib.compress produces a zlib stream (2-byte header + adler32 trailer).
    # System.IO.Compression.DeflateStream expects a *raw* DEFLATE stream
    # (no zlib header/trailer). Use wbits=-15 to get raw deflate.
    c = zlib.compressobj(zlib.Z_DEFAULT_COMPRESSION, zlib.DEFLATED, -15)
    out = c.compress(data) + c.flush()
    return out


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            chunk = f.read(1 << 20)
            if not chunk:
                break
            h.update(chunk)
    return h.digest()


def md5_data(data):
    return hashlib.md5(data).digest()


def main():
    if len(sys.argv) != 4:
        print(__doc__)
        sys.exit(2)
    game_dir = sys.argv[1]
    patch_dir = sys.argv[2]
    out_path = sys.argv[3]

    if not os.path.isdir(game_dir):
        print("game_dir not found: " + game_dir, file=sys.stderr)
        sys.exit(1)
    if not os.path.isdir(patch_dir):
        print("patch_dir not found: " + patch_dir, file=sys.stderr)
        sys.exit(1)

    # ---- collect targets from patch_dir (mode=1 replace) ----
    targets = []  # list of dicts: {path, mode, final_size, sha256, records}
    for root, _dirs, files in os.walk(patch_dir):
        for fn in files:
            full = os.path.join(root, fn)
            rel = os.path.relpath(full, patch_dir).replace(os.sep, "/")
            data = open(full, "rb").read()
            targets.append(
                {
                    "path": rel,
                    "mode": 1,
                    "final_size": len(data),
                    "sha256": hashlib.sha256(data).digest(),
                    "records": [
                        {
                            "path": rel,
                            "src": 1,
                            "payload_data": data,  # to be deflated into blob
                            "md5": md5_data(data),
                        }
                    ],
                }
            )

    # ---- optional mode=0 rebuild target from base_source.bin ----
    base_src = os.path.join(game_dir, "base_source.bin")
    if os.path.isfile(base_src):
        with open(base_src, "rb") as f:
            base_data = f.read(64)
        rebuilt = base_data  # final content = first 64 bytes of base
        targets.append(
            {
                "path": "rebuilt.bin",
                "mode": 0,
                "final_size": len(rebuilt),
                "sha256": hashlib.sha256(rebuilt).digest(),
                "records": [
                    {
                        "path": "rebuilt.bin:copy",
                        "src": 0,
                        "base_path": "base_source.bin",
                        "offset": 0,
                        "size": len(base_data),
                        "md5": md5_data(base_data),
                    }
                ],
            }
        )

    if not targets:
        print("no target files found in patch_dir", file=sys.stderr)
        sys.exit(1)

    # ---- build blob + assign payload offsets ----
    blob = bytearray()
    for t in targets:
        for rec in t["records"]:
            if rec["src"] == 1:
                raw = rec["payload_data"]
                z = deflate_raw(raw)
                rec["payload_ofs"] = len(blob)
                rec["z_size"] = len(z)
                rec["raw_size"] = len(raw)
                blob += z
                # drop payload_data (not needed for header)
                rec.pop("payload_data", None)

    # ---- write header ----
    out = bytearray()
    out += b"FFP1"
    out += struct.pack("<I", 1)  # version

    meta = {
        "patch_name": "FarsiForge Test Patch",
        "game_name": "TestGame",
        "game_exe": "TestGame.exe",
        "engine": "TestEngine",
        "patch_version": "1.0.0",
        "author": "FarsiForge",
        "description": "Auto-generated test patch",
        "created_at": datetime.datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ"),
    }
    for k in (
        "patch_name",
        "game_name",
        "game_exe",
        "engine",
        "patch_version",
        "author",
        "description",
        "created_at",
    ):
        write_str(out, meta[k])

    out += struct.pack("<I", len(targets))
    out += struct.pack("<q", len(blob))  # blob_size (i64)

    for t in targets:
        write_str(out, t["path"])
        out += struct.pack("<B", t["mode"])
        out += struct.pack("<q", t["final_size"])
        out += t["sha256"]
        out += struct.pack("<I", len(t["records"]))
        for rec in t["records"]:
            write_str(out, rec["path"])
            out += struct.pack("<B", rec["src"])
            if rec["src"] == 0:
                write_str(out, rec["base_path"])
                out += struct.pack("<q", rec["offset"])
                out += struct.pack("<q", rec["size"])
            else:
                out += struct.pack("<q", rec["payload_ofs"])
                out += struct.pack("<q", rec["z_size"])
                out += struct.pack("<q", rec["raw_size"])
            out += rec["md5"]

    # ---- write blob ----
    out += blob

    with open(out_path, "wb") as f:
        f.write(out)

    print(
        "wrote %s (%d bytes, %d targets, blob %d bytes)"
        % (out_path, len(out), len(targets), len(blob))
    )


if __name__ == "__main__":
    main()
