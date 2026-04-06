@echo off
REM Упаковать release\SaturX в ZIP без пересборки
powershell -ExecutionPolicy Bypass -File "%~dp0zip-saturx.ps1"
pause
