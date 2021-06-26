@echo off
set "steamcmdpath=C:\steamcmd"
set "serverpath=C:\dayz"
set /p login=Steam Login: 
echo.
set /p pass=Steam Password:
echo.
set /p workshopid=Workshop id:
echo.
set /p modname=Mod folder name:

echo.
%steamcmdpath%\steamcmd +login %login% %pass% +force_install_dir "%serverpath%" +workshop_download_item 221100 %workshopid% validate +quit
move C:\dayz\steamapps\workshop\content\221100\%workshopid% C:\dayz\%modname%
copy C:\dayz\%modname%\Keys\*.bikey C:\dayz\keys

pause
