@echo off
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0KnowledgeBase.ps1" -Action Stop
pause
