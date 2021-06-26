@echo off
:start

set serverName="Epsteins Island US-1 | PVP | Trader | BaseBuilding+ | UnlimtedStamina | DogShitGames.com"
set serverLocation="C:\dayz"
set serverPort=2302
set serverConfig=serverDZ.cfg
set serverCPU=4

title %serverName% batch
cd "%serverLocation%"
start "DayZ Server" /min "DZSALModServer.exe" -mod=@CF -config=%serverConfig% -port=%serverPort% -cpuCount=%serverCPU% -dologs -adminlog -netlog -freezecheck
echo (%time%) %serverName% started.

timeout 14390
taskkill /im DZSALModServer.exe /F
timeout 10

goto start