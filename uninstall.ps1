# uninstall.ps1 — removes the Desktop shortcut and the built binary.
[CmdletBinding()]
param(
    [string]$ShortcutName = "Anki Deck Builder"
)

$ErrorActionPreference = "Stop"

$Repo = $PSScriptRoot
$Desktop = [Environment]::GetFolderPath("Desktop")
$LnkPath = Join-Path $Desktop "$ShortcutName.lnk"
$Exe = Join-Path $Repo "bin\anki.exe"

if (Test-Path $LnkPath) {
    Remove-Item $LnkPath -Force
    Write-Host "Removed shortcut: $LnkPath" -ForegroundColor Green
} else {
    Write-Host "No shortcut at $LnkPath" -ForegroundColor DarkGray
}

if (Test-Path $Exe) {
    Remove-Item $Exe -Force
    Write-Host "Removed binary: $Exe" -ForegroundColor Green
} else {
    Write-Host "No binary at $Exe" -ForegroundColor DarkGray
}
