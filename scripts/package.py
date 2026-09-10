#!/usr/bin/env python3
"""Package the already-built standalone executable and its documentation."""
import hashlib
import re
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DIST = ROOT / 'dist'
VERSION = '1.0.3'


def write_zip(path, entries):
    with zipfile.ZipFile(path, 'w', zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for name, data in entries:
            archive.writestr(name, data)
    with zipfile.ZipFile(path) as archive:
        assert archive.testzip() is None
    print(f'{path.name}: {path.stat().st_size:,} bytes')


def main():
    exe = DIST / 'FloatReader.exe'
    if not exe.is_file():
        raise SystemExit('Build dist/FloatReader.exe first')
    guide = (ROOT / 'docs/使用说明.md').read_text(encoding='utf-8')
    guide = re.sub(r'^#+ ', '', guide, flags=re.M).replace('**', '').replace('`', '')
    guide = ('\ufeff' + guide).replace('\n', '\r\n').encode('utf-8')
    (DIST / '使用说明.txt').write_bytes(guide)
    app_zip = DIST / f'隅读-{VERSION}-Windows-x64.zip'
    write_zip(app_zip, [
        ('FloatReader.exe', exe.read_bytes()),
        ('使用说明.txt', guide),
        ('示例小说-山海来信.txt', (ROOT / 'assets/示例小说-山海来信.txt').read_bytes()),
    ])
    sources = []
    for folder in ('cmd', 'internal', 'assets', 'scripts', 'docs'):
        for path in sorted((ROOT / folder).rglob('*')):
            if path.is_file() and '__pycache__' not in path.parts:
                sources.append((str(path.relative_to(ROOT)), path.read_bytes()))
    for filename in ('go.mod', 'README.md', '.gitignore', '.gitattributes'):
        sources.append((filename, (ROOT / filename).read_bytes()))
    source_zip = DIST / f'隅读-{VERSION}-源码.zip'
    write_zip(source_zip, sources)
    lines = [f'{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.name}' for p in (exe, app_zip, source_zip)]
    (DIST / 'SHA256SUMS.txt').write_text('\n'.join(lines) + '\n', encoding='utf-8')


if __name__ == '__main__':
    main()
