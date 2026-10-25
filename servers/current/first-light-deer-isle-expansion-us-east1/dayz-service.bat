@echo off
REM dzdeploy owns updates, overlay application, and process supervision.
REM Configure deploy.yaml and bootstrap this host before using this entry point.
sc.exe query dzdeploy-deer-isle | findstr /C:"RUNNING" >nul
if not errorlevel 1 exit /b 0
sc.exe start dzdeploy-deer-isle
exit /b %ERRORLEVEL%
