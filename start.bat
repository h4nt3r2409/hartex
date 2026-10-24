@echo off
rem Switch the console to UTF-8; otherwise non-ASCII text in this file is garbled in a GBK terminal.
chcp 65001 >nul 2>&1
rem ARTEX daemon start script (Windows)
rem
rem Usage:
rem   start.bat                  Run in the foreground (Ctrl-C to stop)
rem   start.bat -addr :9000      Extra arguments are passed through to artex unchanged
rem
rem This script does one thing: run artex.exe, then decide whether to restart it based on the exit code when the process exits.
rem
rem   0      User stopped normally   -> end the loop
rem   75     Program requested a restart -> restart immediately ("One-click update" or "Rollback" was clicked in the UI)
rem   other  Abnormal exit         -> restart after a backoff (1->2->4... up to 60 seconds)
rem
rem Downloading, SHA256 checksum verification, and version replacement are not done here; artex handles all of it on startup
rem (selfupdate package). The script is kept simple. See the top of start.sh for details.

setlocal enabledelayedexpansion
cd /d "%~dp0"

set "BIN=artex.exe"
if not exist "%BIN%" (
	echo [artex] Executable not found: %BIN% 1>&2
	exit /b 1
)

set "RESTART_CODE=75"
set "MAX_DELAY=60"
set /a delay=1

:loop
"%BIN%" %*
set "code=!ERRORLEVEL!"

if "!code!"=="0" (
	echo [artex] Exited normally
	exit /b 0
)

if "!code!"=="%RESTART_CODE%" (
	rem Update/rollback is ready. Running again lets artex complete the version replacement on startup.
	echo [artex] Restart requested: applying the new version...
	set /a delay=1
	goto loop
)

echo [artex] Abnormal exit ^(code=!code!^), restarting in !delay!s 1>&2
rem timeout fails in a redirected console, so use ping instead (an N-second delay needs N+1 pings).
set /a pings=!delay!+1
ping -n !pings! 127.0.0.1 >nul 2>&1
set /a delay=!delay!*2
if !delay! gtr %MAX_DELAY% set /a delay=%MAX_DELAY%
goto loop
