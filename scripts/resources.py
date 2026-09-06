#!/usr/bin/env python3
"""Embed the icon, manifest, and version in an AMD64 .syso file for Go builds."""
import struct
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
pack = struct.pack


def align(data, n=4):
    return data + b"\0" * ((-len(data)) % n)


def wide(s):
    return (s + "\0").encode("utf-16le")


def block(key, value=b"", children=(), kind=1, value_length=None):
    length = len(value) // 2 if kind else len(value)
    if value_length is not None:
        length = value_length
    data = align(pack("<HHH", 0, length, kind) + wide(key)) + value
    for child in children:
        data = align(data) + child
    return pack("<H", len(data)) + data[2:]


def version():
    fixed = pack("<13I", 0xFEEF04BD, 0x10000, 0x10000, 0x10000, 0x10000, 0x10000,
                 0x3F, 0, 0x40004, 1, 0, 0, 0)
    fields = {
        "FileDescription": "隅读 · 无边框悬浮小说阅读器",
        "FileVersion": "1.0.1.0",
        "InternalName": "FloatReader",
        "OriginalFilename": "FloatReader.exe",
        "ProductName": "隅读 · Float Reader",
        "ProductVersion": "1.0.1",
        "LegalCopyright": "Copyright © 2026 Float Reader",
    }
    strings = block("StringFileInfo", children=[block("080404B0", children=[
        block(k, wide(v)) for k, v in fields.items()
    ])])
    var = block("VarFileInfo", children=[block("Translation", pack("<HH", 0x804, 1200), kind=0)])
    return block("VS_VERSION_INFO", fixed, [strings, var], kind=0)


def resources():
    ico = (ROOT / "assets/app.ico").read_bytes()
    count = struct.unpack_from("<H", ico, 4)[0]
    group = pack("<HHH", 0, 1, count)
    icons = {}
    for i in range(count):
        entry = ico[6+i*16:22+i*16]
        size, offset = struct.unpack_from("<II", entry, 8)
        icons[i+1] = {0x409: ico[offset:offset+size]}
        group += entry[:12] + pack("<H", i+1)
    return {
        3: icons,
        14: {1: {0x409: group}},
        16: {1: {0x804: version()}},
        24: {1: {0x409: (ROOT / "assets/app.manifest").read_bytes()}},
    }


def build(tree):
    data = bytearray()
    leaves = []

    def directory(node):
        offset = len(data)
        data.extend(pack("<IIHHHH", 0, 0, 0, 0, 0, len(node)))
        data.extend(b"\0" * (8 * len(node)))
        for i, (key, value) in enumerate(sorted(node.items())):
            if isinstance(value, dict):
                target = directory(value) | 0x80000000
            else:
                target = len(data)
                data.extend(b"\0" * 16)
                leaves.append((target, value))
            struct.pack_into("<II", data, offset+16+i*8, key, target)
        return offset

    directory(tree)
    relocs = bytearray()
    for entry, payload in leaves:
        data.extend(b"\0" * ((-len(data)) % 4))
        offset = len(data)
        data.extend(payload)
        struct.pack_into("<IIII", data, entry, offset, len(payload), 0, 0)
        relocs.extend(pack("<IIH", entry, 0, 3))  # IMAGE_REL_AMD64_ADDR32NB
    data = align(data)
    raw_start = 60
    reloc_start = raw_start + len(data)
    symbols_start = reloc_start + len(relocs)
    header = pack("<HHIIIHH", 0x8664, 1, 0, symbols_start, 1, 0, 0)
    section = pack("<8sIIIIIIHHI", b".rsrc\0\0\0", 0, 0, len(data), raw_start,
                   reloc_start, 0, len(leaves), 0, 0x40000040)
    symbol = pack("<8sIhHBB", b".rsrc\0\0\0", 0, 1, 0, 3, 0)
    return header + section + data + relocs + symbol + pack("<I", 4)


if __name__ == "__main__":
    path = ROOT / "cmd/floatreader/resources_windows_amd64.syso"
    path.write_bytes(build(resources()))
    print(f"Created {path.name}: {path.stat().st_size:,} bytes")
