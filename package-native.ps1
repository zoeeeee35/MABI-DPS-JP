param([string]$Go='go',[string]$OutRoot=(Join-Path $PSScriptRoot 'release-output'))
$ErrorActionPreference='Stop'
& (Join-Path $PSScriptRoot 'app/package-native.ps1') -Go $Go -OutRoot $OutRoot
