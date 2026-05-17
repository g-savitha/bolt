# Install bolt from GitHub Releases (Windows).
# Usage:
#   irm https://raw.githubusercontent.com/g-savitha/bolt/main/scripts/install.ps1 | iex
#   .\scripts\install.ps1 [-Version 0.1.1]
param(
    [string]$Version = "",
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\bolt"
)

$ErrorActionPreference = "Stop"
$Owner = "g-savitha"
$Repo = "bolt"

# Resolve version: explicit param > latest release from GitHub API
if ($Version -eq "") {
    $ApiUrl = "https://api.github.com/repos/$Owner/$Repo/releases/latest"
    $Release = Invoke-RestMethod -Uri $ApiUrl -UseBasicParsing
    $Version = $Release.tag_name -replace '^v', ''
    if (-not $Version) {
        throw "Could not resolve latest release from GitHub API"
    }
}

$Asset = "bolt_${Version}_windows_amd64.zip"
$Url = "https://github.com/$Owner/$Repo/releases/download/v$Version/$Asset"

$TempDir = Join-Path $env:TEMP "bolt-install"
New-Item -ItemType Directory -Force -Path $TempDir | Out-Null
$ZipPath = Join-Path $TempDir $Asset

Write-Host "Downloading $Url ..."
Invoke-WebRequest -Uri $Url -OutFile $ZipPath -UseBasicParsing

Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force
$Exe = Join-Path $TempDir "bolt.exe"
if (-not (Test-Path $Exe)) {
    throw "archive did not contain bolt.exe"
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Copy-Item -Force $Exe (Join-Path $InstallDir "bolt.exe")

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
}

Write-Host "Installed bolt $Version to $InstallDir\bolt.exe"
Write-Host "Open a new terminal and run: bolt init"
