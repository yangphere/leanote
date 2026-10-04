#Requires -Version 5.1
<#
.SYNOPSIS
Build the local Leanote image from the current repository sources.
.DESCRIPTION
Uses docker-compose.yml and docker-compose.dev.yml together to build
leanote:local. Compose reads the selected env file and environment variables,
including LEANOTE_VERSION. Run with Docker Desktop in Linux container mode.
.PARAMETER EnvFile
Compose env file, relative to the repository root or an absolute path.
Defaults to .env. Use .env.example to build with the sample local version.
.PARAMETER Pull
Pull the pinned base images before building.
.PARAMETER NoCache
Build without using cached layers.
.EXAMPLE
.\build-local-image.ps1
.EXAMPLE
.\build-local-image.ps1 -Pull -NoCache
.EXAMPLE
.\build-local-image.ps1 -EnvFile .env.example
#>
[CmdletBinding()]
param(
    [ValidateNotNullOrEmpty()]
    [string]$EnvFile = '.env',
    [switch]$Pull,
    [switch]$NoCache
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$dockerCommand = Get-Command docker -CommandType Application -ErrorAction Stop |
    Select-Object -First 1
$environmentPath = if ([System.IO.Path]::IsPathRooted($EnvFile)) {
    $EnvFile
}
else {
    Join-Path $PSScriptRoot $EnvFile
}
$buildArguments = @(
    'compose'
    '--env-file'
    $environmentPath
    '-f'
    (Join-Path $PSScriptRoot 'docker-compose.yml')
    '-f'
    (Join-Path $PSScriptRoot 'docker-compose.dev.yml')
    'build'
)
if ($Pull) {
    $buildArguments += '--pull'
}
if ($NoCache) {
    $buildArguments += '--no-cache'
}
$buildArguments += 'leanote'

Push-Location -LiteralPath $PSScriptRoot
try {
    & $dockerCommand.Source @buildArguments
    if ($LASTEXITCODE -ne 0) {
        throw "Local image build failed (Docker exit code: $LASTEXITCODE)."
    }
    Write-Host 'Local image ready: leanote:local'
}
finally {
    Pop-Location
}
