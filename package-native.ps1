param([string]$Go='go',[string]$OutRoot=(Join-Path $PSScriptRoot '../../releases'))
$ErrorActionPreference='Stop'
$version=[IO.File]::ReadAllText((Join-Path $PSScriptRoot 'VERSION')).Trim()
if($version -notmatch '^\d+\.\d+\.\d+$'){throw 'Invalid VERSION'}
New-Item -ItemType Directory -Path $OutRoot -Force | Out-Null
$OutRoot=(Resolve-Path $OutRoot).Path
$out=Join-Path $OutRoot ('MABI-DPS-JP-v'+$version+'-Windows')
$zip=$out+'.zip'
# Never silently replace a published release.
if((Test-Path -LiteralPath $out) -or (Test-Path -LiteralPath $zip)){throw 'This release already exists. Use a new VERSION or output root.'}
& (Join-Path $PSScriptRoot 'build-native.ps1') -Go $Go -OutDir $out
$allowed=@('MabiDPS-JP.exe','識別子翻訳.csv','config','licenses','sounds','効果音クレジット.txt','sample-combat.json','Paperlogy-OFL.txt','RoundedMplus-OFL.txt','VERSION','使い方.txt','LICENSE','THIRD-PARTY.md','release-manifest.json')
$files=@($allowed | ForEach-Object { Join-Path $out $_ })
Compress-Archive -LiteralPath $files -DestinationPath $zip
Write-Host "配布ZIP: $zip"

