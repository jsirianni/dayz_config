@echo off
call :work arg
goto :EOF
:work
echo %~1
goto :EOF
