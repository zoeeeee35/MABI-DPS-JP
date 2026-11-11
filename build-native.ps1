param(
 [string]$Go = "go",
 [string]$OutDir = (Join-Path $PSScriptRoot "dist-native"),
 [switch]$SkipGuiTest
)
$ErrorActionPreference = "Stop"
Push-Location $PSScriptRoot
try {
 & (Join-Path $PSScriptRoot "build-icon.ps1")
 & $Go test -tags native ./...
 if ($LASTEXITCODE -ne 0) { throw "自動テストが失敗しました" }
 & $Go vet -tags native ./...
 if ($LASTEXITCODE -ne 0) { throw "静的検査が失敗しました" }
 New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
 $OutDir = (Resolve-Path $OutDir).Path
 $exePath = Join-Path $OutDir "MabiDPS-JP.exe"
 $version = ([IO.File]::ReadAllText((Join-Path $PSScriptRoot "VERSION"))).Trim()
 $buildUTC = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
 $seedHash = (Get-FileHash -LiteralPath (Join-Path $PSScriptRoot "スキル翻訳.csv") -Algorithm SHA256).Hash.ToLowerInvariant()
 & $Go build -tags native -trimpath -ldflags "-H=windowsgui -X main.buildUTC=$buildUTC -X main.translationSeedSHA256=$seedHash" -o $exePath .
 if ($LASTEXITCODE -ne 0) { throw "ビルドが失敗しました" }
 $translationPath = Join-Path $OutDir "スキル翻訳.csv"
 if (-not (Test-Path -LiteralPath $translationPath)) { Copy-Item -LiteralPath "スキル翻訳.csv" -Destination $translationPath }
 if (-not (Test-Path -LiteralPath (Join-Path $OutDir '効果ID翻訳.csv'))) { Copy-Item -LiteralPath '効果ID翻訳.csv' -Destination $OutDir }
 Copy-Item -LiteralPath "native_sample.json" -Destination (Join-Path $OutDir "sample-combat.json") -Force
 Copy-Item -LiteralPath "config" -Destination $OutDir -Recurse -Force
 Copy-Item -LiteralPath "licenses" -Destination $OutDir -Recurse -Force
 Copy-Item -LiteralPath "sounds" -Destination $OutDir -Recurse -Force
 Copy-Item -LiteralPath "効果音クレジット.txt" -Destination $OutDir -Force
 Copy-Item -LiteralPath "fonts/OFL.txt" -Destination (Join-Path $OutDir "Paperlogy-OFL.txt") -Force
 Copy-Item -LiteralPath "fonts/RoundedMplus-OFL.txt" -Destination (Join-Path $OutDir "RoundedMplus-OFL.txt") -Force
 if (-not $SkipGuiTest) {
  $reportPath = Join-Path $OutDir "ui-self-test.json"
  $testDir = Join-Path $OutDir ("self-test-data-" + [Guid]::NewGuid().ToString("N"))
  $started = [DateTime]::UtcNow
  $proc = Start-Process -FilePath $exePath -ArgumentList "-ui-self-test",('"'+$reportPath+'"'),"-data-dir",('"'+$testDir+'"') -WindowStyle Hidden -PassThru
  if (-not $proc.WaitForExit(30000)) { $proc.Kill(); throw "GUI自動テストが制限時間内に終了しませんでした" }
  if ($proc.ExitCode -ne 0) { throw "GUI自動テストが異常終了しました" }
  if ((Get-Item -LiteralPath $reportPath).LastWriteTimeUtc -lt $started) { throw "GUI検証結果が更新されていません" }
  $report = Get-Content -LiteralPath $reportPath -Raw | ConvertFrom-Json
  if (-not $report.layered -or -not $report.topmost -or -not $report.noActivate -or -not $report.clickThrough -or -not $report.layerAttributesApplied -or $report.opacity -ne 220 -or $report.previewError -ne "" -or $report.liveVerified -or $report.historyCount -ne 2) { throw "GUI自動テストの状態が期待値と一致しません" }
  if ($report.result.battles[0].damage -ne 22300 -or $report.result.battles[1].damage -ne 9500) { throw "GUI再生時の合計ダメージが一致しません" }
  $leaders = @($report.trace | Where-Object { $_.battle -eq 1 -and $_.active } | ForEach-Object { $_.players[0].id })
  if (($leaders -join ',') -ne 'sample:a,sample:b,sample:a,sample:b,sample:a') { throw "サンプルの順位更新が期待値と一致しません" }
 }
 foreach ($file in @('VERSION','TRANSLATIONS_VERSION','使い方.txt','LICENSE','THIRD-PARTY.md')) { Copy-Item -LiteralPath $file -Destination $OutDir -Force }
 $actualHash = (Get-FileHash -LiteralPath $translationPath -Algorithm SHA256).Hash.ToLowerInvariant()
 $manifest = [ordered]@{ application='MABI DPS JP'; version=$version; buildUTC=$buildUTC; executable='MabiDPS-JP.exe'; executableSHA256=(Get-FileHash -LiteralPath $exePath -Algorithm SHA256).Hash.ToLowerInvariant(); effectTranslationCSV_SHA256=(Get-FileHash -LiteralPath (Join-Path $OutDir "効果ID翻訳.csv") -Algorithm SHA256).Hash.ToLowerInvariant(); translationRevision=([IO.File]::ReadAllText((Join-Path $PSScriptRoot 'TRANSLATIONS_VERSION'))).Trim(); translationSeedSHA256=$seedHash; translationCSV_SHA256=$actualHash; translationEdited=($actualHash -ne $seedHash) }
 $manifest | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $OutDir 'release-manifest.json') -Encoding utf8
 $pe = (Get-Item -LiteralPath $exePath).VersionInfo
 if ($pe.FileVersion -ne $version -or $pe.ProductVersion -ne $version) { throw 'EXE version resource mismatch' }
 Write-Host "ビルド・検証完了: $exePath / v$version"
} finally { Pop-Location }

