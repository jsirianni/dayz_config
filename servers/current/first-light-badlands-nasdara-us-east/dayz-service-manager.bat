@echo off
REM Compatibility entry point. The Windows service owns the restart loop.
call "%~dp0dayz-service.bat"
exit /b %ERRORLEVEL%
