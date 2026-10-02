# Builds FastLED (master, stub/host platform) as a static set of objects with zig c++,
# then links the given reference program(s). Used only for the feasibility study.
param([string[]]$Programs = @('spike/ref-native-math.cpp'))
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
$env:ZIG_GLOBAL_CACHE_DIR = "$PSScriptRoot\.zig-cache"
$env:ZIG_LOCAL_CACHE_DIR = "$PSScriptRoot\.zig-cache"
$zig = "$PSScriptRoot\.venv\Lib\site-packages\ziglang\zig.exe"
$flags = @('-std=gnu++17', '-O2', '-DSTUB_PLATFORM', '-DFASTLED_STUB_IMPL', '-DFASTLED_USE_STUB_ARDUINO', '-DFASTLED_TESTING', '-Ifastled/src', '-w')
New-Item -ItemType Directory -Force build | Out-Null

$units = Get-ChildItem fastled/src/fl/build -Filter *.cpp
Remove-Item build/*.err -ErrorAction SilentlyContinue
$running = @()
foreach ($u in $units) {
  $obj = "build/" + ($u.BaseName -replace '[+]', '') + ".o"
  if ((Test-Path $obj) -and ((Get-Item $obj).LastWriteTime -gt $u.LastWriteTime)) { continue }
  while (($running | Where-Object { -not $_.HasExited }).Count -ge 3) { Start-Sleep -Milliseconds 500 }
  $args = @('c++') + $flags + @('-c', "`"$($u.FullName)`"", '-o', $obj)
  $running += Start-Process -FilePath $zig -ArgumentList $args -NoNewWindow -PassThru -RedirectStandardError "build/$($u.BaseName).err"
}
$running | Wait-Process
$failed = Get-ChildItem build -Filter *.err | Where-Object { $_.Length -gt 0 }
foreach ($f in $failed) { Write-Host "== $($f.Name)"; Get-Content $f.FullName -TotalCount 15 }

$objs = Get-ChildItem build -Filter *.o | ForEach-Object { $_.FullName }
foreach ($p in $Programs) {
  $exe = "build/" + [IO.Path]::GetFileNameWithoutExtension($p) + ".exe"
  & $zig c++ @flags $p @objs -lws2_32 -lwinmm -lbcrypt -ladvapi32 -o $exe 2>&1 | Select-Object -First 30
  Write-Host "built $exe"
}
