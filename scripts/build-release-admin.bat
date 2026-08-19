@echo off
REM Admin release (no license). From project root: scripts\build-release-admin.bat
powershell -ExecutionPolicy Bypass -File "%~dp0build-release-admin.ps1"
pause
