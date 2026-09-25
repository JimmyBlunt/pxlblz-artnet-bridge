@echo off
setlocal
cd /d "%~dp0.."

set PROFILE=%~1
if "%PROFILE%"=="" set PROFILE=smoke

set SECONDS=%~2
if "%SECONDS%"=="" (
  node perf-test\run-performance.mjs --profile %PROFILE%
) else (
  node perf-test\run-performance.mjs --profile %PROFILE% --seconds %SECONDS%
)

exit /b %ERRORLEVEL%
