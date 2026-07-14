#!/usr/bin/env python3
"""Generate inert DataGuardian demo fixtures. No sample contains executable code."""

from __future__ import annotations

import argparse
import base64
import hashlib
import struct
import zlib
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SAMPLES = ROOT / "samples"
MARKERS = b"eval(demo_only) " + (b"A" * 96)


def pdf(comment: bytes = b"") -> bytes:
    objects = [
        b"1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj",
        b"2 0 obj << /Type /Pages /Kids [3 0 R] /Count 1 >> endobj",
        b"3 0 obj << /Type /Page /Parent 2 0 R /MediaBox [0 0 300 120] /Contents 4 0 R >> endobj",
        b"4 0 obj << /Length 0 >> stream\n\nendstream endobj",
    ]
    data = b"%PDF-1.4\n% DataGuardian inert fixture\n" + (b"% " + comment + b"\n" if comment else b"")
    offsets = [0]
    for obj in objects:
        offsets.append(len(data)); data += obj + b"\n"
    xref = len(data)
    data += b"xref\n0 5\n0000000000 65535 f \n"
    data += b"".join(f"{value:010d} 00000 n \n".encode() for value in offsets[1:])
    return data + f"trailer << /Size 5 /Root 1 0 R >>\nstartxref\n{xref}\n%%EOF\n".encode()


def png(comment: bytes = b"") -> bytes:
    def chunk(kind: bytes, payload: bytes) -> bytes:
        return struct.pack(">I", len(payload)) + kind + payload + struct.pack(">I", zlib.crc32(kind + payload) & 0xFFFFFFFF)
    signature = b"\x89PNG\r\n\x1a\n"
    ihdr = chunk(b"IHDR", struct.pack(">IIBBBBB", 1, 1, 8, 2, 0, 0, 0))
    text = chunk(b"tEXt", b"Comment\x00" + comment) if comment else b""
    image = chunk(b"IDAT", zlib.compress(b"\x00\x33\x99\xcc"))
    return signature + ihdr + text + image + chunk(b"IEND", b"")


JPEG_1X1 = base64.b64decode(
    b"/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAP//////////////////////////////////////////////////////////////////////////////////////2wBDAf//////////////////////////////////////////////////////////////////////////////////////wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAX/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIQAxAAAAEf/8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQABBQJ//8QAFBEBAAAAAAAAAAAAAAAAAAAAAP/aAAgBAwEBPwF//8QAFBEBAAAAAAAAAAAAAAAAAAAAAP/aAAgBAgEBPwF//8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQAGPwJ//8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQABPyF//9oADAMBAAIAAwAAABAf/8QAFBEBAAAAAAAAAAAAAAAAAAAAAP/aAAgBAwEBPxB//8QAFBEBAAAAAAAAAAAAAAAAAAAAAP/aAAgBAgEBPxB//8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQABPxB//9k="
)


def jpeg(comment: bytes = b"") -> bytes:
    if not comment:
        return JPEG_1X1
    segment = b"\xff\xfe" + struct.pack(">H", len(comment) + 2) + comment
    return JPEG_1X1[:2] + segment + JPEG_1X1[2:]


def jpeg_with_fictional_exif() -> bytes:
    tiff = bytearray(228)
    tiff[0:2] = b"II"
    struct.pack_into("<H", tiff, 2, 42)
    struct.pack_into("<I", tiff, 4, 8)
    struct.pack_into("<H", tiff, 8, 3)
    def entry(offset: int, tag: int, field_type: int, count: int, value: int) -> None:
        struct.pack_into("<HHII", tiff, offset, tag, field_type, count, value)
    entry(10, 0x0110, 2, 8, 80)
    entry(22, 0x0132, 2, 20, 96)
    entry(34, 0x8825, 4, 1, 128)
    tiff[80:88] = b"DemoCam\x00"
    tiff[96:116] = b"2026:01:01 00:00:00\x00"
    struct.pack_into("<H", tiff, 128, 4)
    entry(130, 1, 2, 2, ord("N"))
    entry(142, 2, 5, 3, 180)
    entry(154, 3, 2, 2, ord("W"))
    entry(166, 4, 5, 3, 204)
    for offset, numerator in [(180, 40), (188, 30), (196, 0), (204, 3), (212, 10), (220, 0)]:
        struct.pack_into("<II", tiff, offset, numerator, 1)
    payload = b"Exif\x00\x00" + bytes(tiff)
    segment = b"\xff\xe1" + struct.pack(">H", len(payload) + 2) + payload
    return JPEG_1X1[:2] + segment + JPEG_1X1[2:]


def build() -> dict[str, bytes]:
    return {
        "clean/clean.pdf": pdf(),
        "clean/clean.png": png(),
        "clean/clean.jpg": jpeg(),
        "clean/clean.txt": b"DataGuardian clean demonstration text.\n",
        "suspicious-inert/pdf-js-markers.pdf": pdf(b"/OpenAction /JS JavaScript DEMO_ONLY_NOT_AN_ACTION " + MARKERS),
        "suspicious-inert/png-encoded-marker.png": png(b"DEMO_ONLY " + MARKERS),
        "suspicious-inert/jpeg-encoded-marker.jpg": jpeg(b"DEMO_ONLY " + MARKERS),
        "suspicious-inert/jpeg-exif-gps.jpg": jpeg_with_fictional_exif(),
        "suspicious-inert/text-eval-marker.txt": b"Synthetic inert marker: " + MARKERS + b"\n",
        "rejected/mismatched-extension.jpg": png(b"PNG content intentionally named JPG"),
        "rejected/malformed.pdf": b"This is deliberately not a PDF.\n",
        "rejected/empty.txt": b"",
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    files = build()
    checksums = "".join(f"{hashlib.sha256(data).hexdigest()}  {name}\n" for name, data in sorted(files.items())).encode()
    files["CHECKSUMS.sha256"] = checksums
    if args.check:
        mismatches = [name for name, data in files.items() if not (SAMPLES / name).is_file() or (SAMPLES / name).read_bytes() != data]
        if mismatches:
            print("Sample mismatch:", ", ".join(mismatches)); return 1
        print("Safe sample corpus is reproducible."); return 0
    for name, data in files.items():
        path = SAMPLES / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)
    print(f"Generated {len(files) - 1} inert fixtures and checksums.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
