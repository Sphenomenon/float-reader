$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')
New-Item -ItemType Directory -Force dist | Out-Null
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Tests failed' }
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
go vet -unsafeptr=false ./...
if ($LASTEXITCODE -ne 0) { throw 'Static checks failed' }
go build -buildvcs=false -trimpath -ldflags '-H windowsgui -s -w' -o dist/FloatReader.exe ./cmd/floatreader
if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
Write-Host 'Created dist/FloatReader.exe'
Write-Host 'Optional: python scripts/package.py to make the customer and source ZIP packages.'
