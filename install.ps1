# install.ps1 — builds the Anki TUI and puts a launcher shortcut on the Desktop.
#
# Run from anywhere:
#     powershell -ExecutionPolicy Bypass -File install.ps1
# or, if execution policy already allows it, just:
#     .\install.ps1
#
# It compiles cmd/anki into bin\anki.exe and creates an "Anki Deck Builder"
# shortcut on your Desktop. The shortcut's working directory is this repo, so the
# offline dictionary at data\de-compact.jsonl resolves at runtime. Double-click
# the shortcut to open the TUI in a console window.

[CmdletBinding()]
param(
    # Override the shortcut name if you like.
    [string]$ShortcutName = "Anki Deck Builder"
)

$ErrorActionPreference = "Stop"

# Resolve repo root to this script's location so it works no matter the cwd.
$Repo = $PSScriptRoot
$BinDir = Join-Path $Repo "bin"
$Exe = Join-Path $BinDir "anki.exe"

Write-Host "Building anki.exe ..." -ForegroundColor Cyan
if (-not (Test-Path $BinDir)) { New-Item -ItemType Directory -Path $BinDir | Out-Null }

Push-Location $Repo
try {
    & go build -o $Exe ./cmd/anki
    if ($LASTEXITCODE -ne 0) { throw "go build failed (exit $LASTEXITCODE)" }
}
finally {
    Pop-Location
}

if (-not (Test-Path $Exe)) { throw "build reported success but $Exe is missing" }
Write-Host "Built $Exe" -ForegroundColor Green

# Create the Desktop shortcut.
$Desktop = [Environment]::GetFolderPath("Desktop")
$LnkPath = Join-Path $Desktop "$ShortcutName.lnk"

$WScript = New-Object -ComObject WScript.Shell
$Shortcut = $WScript.CreateShortcut($LnkPath)
$Shortcut.TargetPath = $Exe
$Shortcut.WorkingDirectory = $Repo          # so data\de-compact.jsonl is found
$Shortcut.Description = "German word -> Anki deck builder (AnkiConnect)"
$Shortcut.IconLocation = "$Exe,0"
$Shortcut.Save()

Write-Host "Shortcut created: $LnkPath" -ForegroundColor Green
Write-Host ""
Write-Host "Done. Make sure Anki is running with the AnkiConnect add-on, then" -ForegroundColor Yellow
Write-Host "double-click `"$ShortcutName`" on your Desktop." -ForegroundColor Yellow
