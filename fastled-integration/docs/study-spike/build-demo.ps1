# Builds and runs the native reference runner for one unmodified FastLED example.
param([Parameter(Mandatory)][string]$Example, [int]$Frames = 1200)
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
$env:ZIG_GLOBAL_CACHE_DIR = "$PSScriptRoot\.zig-cache"
$env:ZIG_LOCAL_CACHE_DIR = "$PSScriptRoot\.zig-cache"
$zig = "$PSScriptRoot\.venv\Lib\site-packages\ziglang\zig.exe"
$name = $Example.ToLower()
$ino = (Resolve-Path "fastled/examples/$Example/$Example.ino").Path.Replace('\', '/')
$cfg = "build/$name.config.h"
@"
#define SKETCH "$ino"
#define OUT_PREFIX "build/$name"
#define FRAMES $Frames
"@ | Set-Content $cfg -Encoding ascii
$flags = @('-std=gnu++17', '-O2', '-DSTUB_PLATFORM', '-DFASTLED_STUB_IMPL', '-DFASTLED_USE_STUB_ARDUINO', '-DFASTLED_TESTING', '-Ifastled/src', '-w')
$objs = Get-ChildItem build -Filter *.o | ForEach-Object { $_.FullName }
& $zig c++ @flags -include $cfg spike/harness-native.cpp @objs -lws2_32 -lwinmm -lbcrypt -ladvapi32 -o "build/$name-native.exe" 2>&1 | Select-Object -First 20
& "build/$name-native.exe"
