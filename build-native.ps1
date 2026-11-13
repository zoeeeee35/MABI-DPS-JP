param([string]$Go='go',[string]$OutDir=(Join-Path $PSScriptRoot 'dist-native'),[switch]$SkipGuiTest)
$ErrorActionPreference='Stop'
& (Join-Path $PSScriptRoot 'app/build-native.ps1') -Go $Go -OutDir $OutDir -SkipGuiTest:$SkipGuiTest
