@echo off
:start

set serverName="Kamyshowdown | Vanilla | DogShitGames.com"
set serverLocation="C:\dayz"
set serverPort=2302
set serverConfig=serverDZ.cfg
set serverCPU=4

title %serverName% batch
cd "%serverLocation%"
c:\steamcmd\steamcmd.exe +login anonymous +force_install_dir %serverLocation% +app_update 223350  +quit
start "DayZ Server" /min "DayZServer_x64.exe" -config=%serverConfig% -port=%serverPort% -cpuCount=%serverCPU% -dologs -adminlog -netlog -freezecheck
echo (%time%) %serverName% started.

timeout 14390
taskkill /im DayZServer_x64.exe /F
timeout 10

goto start
