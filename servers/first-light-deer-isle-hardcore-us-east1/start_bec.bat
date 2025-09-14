@echo off
setlocal enableextensions

echo [%date% %time%] BEC Service Manager starting...
echo [%date% %time%] This will run BEC in a loop with automatic restarts
echo [%date% %time%] Press Ctrl+C to stop the service manager
echo.

:bec_loop
echo [%date% %time%] Starting BEC...
cd /d "C:\DayZ\BEC"
Bec.exe -f Config.cfg --dsc
set "EXITCODE=%ERRORLEVEL%"

echo [%date% %time%] BEC exited with code %EXITCODE%
echo [%date% %time%] Waiting 10 seconds before restart...
timeout /t 10 /nobreak >nul

echo [%date% %time%] Restarting BEC...
echo.
goto :bec_loop