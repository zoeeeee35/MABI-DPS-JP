# Build a native AMD64 COFF resource object from the supplied PNG, without external tools.
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'build-version-resource.ps1')
Add-Type -AssemblyName System.Drawing
$img=[Drawing.Image]::FromFile((Join-Path $PSScriptRoot 'assets/app-icon.png'))
$bmp=[Drawing.Bitmap]::new(256,256)
$g=[Drawing.Graphics]::FromImage($bmp)
$g.InterpolationMode=[Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
$g.DrawImage($img,0,0,256,256)
$ms=[IO.MemoryStream]::new(); $bmp.Save($ms,[Drawing.Imaging.ImageFormat]::Png)
$png=$ms.ToArray(); $g.Dispose(); $bmp.Dispose(); $img.Dispose(); $ms.Dispose()
$stream=[IO.MemoryStream]::new(); $w=[IO.BinaryWriter]::new($stream)
function U16($v){$w.Write([uint16]$v)}
function U32($v){$w.Write([uint32]$v)}
function WriteResourceDirectory {U32 0;U32 0;U16 0;U16 0;U16 0;U16 1}
# Three resource types: icon, icon group, Windows version information.
U32 0;U32 0;U16 0;U16 0;U16 0;U16 3
U32 3;U32 (2147483648+40)
U32 14;U32 (2147483648+88)
U32 16;U32 (2147483648+136)
WriteResourceDirectory;U32 1;U32 (2147483648+64)
WriteResourceDirectory;U32 1033;U32 184
WriteResourceDirectory;U32 1;U32 (2147483648+112)
WriteResourceDirectory;U32 1033;U32 200
WriteResourceDirectory;U32 1;U32 (2147483648+160)
WriteResourceDirectory;U32 1041;U32 216
U32 232;U32 $png.Length;U32 0;U32 0
$groupOffset=232+$png.Length
U32 $groupOffset;U32 20;U32 0;U32 0
# Align version resource to DWORD boundary.
$versionOffset=$groupOffset+20
while($versionOffset%4){$versionOffset++}
U32 $versionOffset;U32 $versionResource.Length;U32 0;U32 0
$w.Write($png)
U16 0;U16 1;U16 1
$w.Write([byte]0);$w.Write([byte]0);$w.Write([byte]0);$w.Write([byte]0)
U16 1;U16 32;U32 $png.Length;U16 1
while($stream.Position%4){$w.Write([byte]0)}
$w.Write([byte[]]$versionResource)
$data=$stream.ToArray(); $w.Dispose();$stream.Dispose()
$out=[IO.MemoryStream]::new();$w=[IO.BinaryWriter]::new($out)
U16 34404;U16 1;U32 0;U32 (60+$data.Length+30);U32 1;U16 0;U16 0
$w.Write([Text.Encoding]::ASCII.GetBytes('.rsrc'+[char]0+[char]0+[char]0))
U32 0;U32 0;U32 $data.Length;U32 60;U32 (60+$data.Length);U32 0;U16 3;U16 0;U32 1073741888
$w.Write($data)
foreach($offset in @(184,200,216)){U32 $offset;U32 0;U16 3}
$w.Write([Text.Encoding]::ASCII.GetBytes('.rsrc'+[char]0+[char]0+[char]0));U32 0;U16 1;U16 0;$w.Write([byte]3);$w.Write([byte]0)
U32 4
[IO.File]::WriteAllBytes((Join-Path $PSScriptRoot 'appicon_windows_amd64.syso'),$out.ToArray())
$w.Dispose();$out.Dispose()
