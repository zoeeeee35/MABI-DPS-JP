# Windows VERSIONINFO resource. VERSION is the single application version source.
$releaseVersion = [IO.File]::ReadAllText((Join-Path $PSScriptRoot 'VERSION')).Trim()
if ($releaseVersion -notmatch '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') { throw 'Invalid VERSION' }
$parts = @($releaseVersion.Split('.') | ForEach-Object { [uint32]$_ })
if (@($parts | Where-Object { $_ -gt 65535 }).Count) { throw 'VERSION component exceeds Windows limit' }
function New-VersionBlock([string]$key,[uint16]$kind,[byte[]]$value,[uint16]$valueLength,[object[]]$children) {
 $b=[IO.MemoryStream]::new();$bw=[IO.BinaryWriter]::new($b)
 $bw.Write([uint16]0);$bw.Write($valueLength);$bw.Write($kind)
 $bw.Write([Text.Encoding]::Unicode.GetBytes($key+[char]0))
 while($b.Position%4){$bw.Write([byte]0)}
 if($value.Length){$bw.Write($value)}
 foreach($child in $children){while($b.Position%4){$bw.Write([byte]0)};$bw.Write([byte[]]$child)}
 $bytes=$b.ToArray();$bw.Dispose();$b.Dispose()
 [BitConverter]::GetBytes([uint16]$bytes.Length).CopyTo($bytes,0)
 return ,$bytes
}
$fixed=[IO.MemoryStream]::new();$fw=[IO.BinaryWriter]::new($fixed)
foreach($n in @([uint32]4277077181,65536,($parts[0]*65536+$parts[1]),($parts[2]*65536),($parts[0]*65536+$parts[1]),($parts[2]*65536),63,0,262148,1,0,0,0)){$fw.Write([uint32]$n)}
$fixedBytes=$fixed.ToArray();$fw.Dispose();$fixed.Dispose()
$strings=[Collections.Generic.List[object]]::new()
foreach($item in @(@('FileDescription','MABI DPS JP'),@('ProductName','MABI DPS JP'),@('FileVersion',$releaseVersion),@('ProductVersion',$releaseVersion),@('OriginalFilename','MabiDPS-JP.exe'))){
 $v=[Text.Encoding]::Unicode.GetBytes($item[1]+[char]0)
 $strings.Add((New-VersionBlock $item[0] 1 $v ($v.Length/2) @()))
}
$table=New-VersionBlock '041104B0' 1 @() 0 $strings.ToArray()
$stringInfo=New-VersionBlock 'StringFileInfo' 1 @() 0 @(,$table)
$translation=New-VersionBlock 'Translation' 0 ([byte[]]@(17,4,176,4)) 4 @()
$varInfo=New-VersionBlock 'VarFileInfo' 1 @() 0 @(,$translation)
$versionResource=New-VersionBlock 'VS_VERSION_INFO' 0 $fixedBytes 52 @($stringInfo,$varInfo)
