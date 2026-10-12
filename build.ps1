# Builds build\TvTray.exe (single file, GUI subsystem, with icon and manifest).
# Linux: go build -trimpath -ldflags "-s -w" -o build/tvtray .  (needs gtk3/webkit2gtk dev packages)
param([string]$Version = "1.0.0")
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot
New-Item -ItemType Directory -Force build | Out-Null

go run ./tools/genicon build/icon.png
go run github.com/tc-hib/go-winres@v0.3.3 simply `
    --arch amd64 --icon build/icon.png --manifest gui `
    --product-name TvTray --file-description "TvTray - Samsung TV control" `
    --file-version $Version --product-version $Version
if ($LASTEXITCODE) { exit $LASTEXITCODE }

go build -trimpath -ldflags "-s -w -H windowsgui" -o build/TvTray.exe .
if ($LASTEXITCODE) { exit $LASTEXITCODE }
Get-Item build/TvTray.exe | ForEach-Object { "{0}  {1:N1} MB" -f $_.FullName, ($_.Length / 1MB) }
