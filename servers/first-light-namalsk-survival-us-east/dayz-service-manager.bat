@echo off
setlocal enableextensions

REM ===== DayZ Service Manager =====
REM This script runs dayz-service.bat in a loop, ensuring mods are synchronized on every restart

echo [%date% %time%] DayZ Service Manager starting...
echo [%date% %time%] This will run DayZ server in a loop with automatic restarts
echo [%date% %time%] Press Ctrl+C to stop the service manager
echo.

:serviceloop
echo [%date% %time%] Starting DayZ service...
call "%~dp0dayz-service.bat"
set "EXITCODE=%ERRORLEVEL%"

echo [%date% %time%] DayZ service exited with code %EXITCODE%
echo [%date% %time%] Waiting 10 seconds before restart...
timeout /t 10 /nobreak >nul

echo [%date% %time%] Restarting DayZ service...
echo.
goto :serviceloop
