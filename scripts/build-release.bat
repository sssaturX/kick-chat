@echo off
REM Run: from project root: scripts\build-release.bat
powershell -ExecutionPolicy Bypass -File "%~dp0build-release.ps1"
pause
