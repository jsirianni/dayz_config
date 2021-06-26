@echo off
set "steamcmdpath=C:\steamcmd"
set "serverpath=C:\dayz"
set /p login=Steam Login: 
echo.
set /p pass=Steam Password:

echo.

echo "installing CF"
%steamcmdpath%\steamcmd +login %login% %pass% +force_install_dir "%serverpath%" +workshop_download_item 221100 1559212036 validate +quit
move C:\dayz\steamapps\workshop\content\221100\1559212036 C:\dayz\@CF
copy C:\dayz\@CF\Keys\*.bikey C:\dayz\keys

echo "installing Community-Online-Tools"
%steamcmdpath%\steamcmd +login %login% %pass% +force_install_dir "%serverpath%" +workshop_download_item 221100 1564026768 validate +quit
move C:\dayz\steamapps\workshop\content\221100\1564026768 C:\dayz\@Community-Online-Tools
copy C:\dayz\@Community-Online-Tools\Keys\*.bikey C:\dayz\keys

echo "installing PartyMe"
%steamcmdpath%\steamcmd +login %login% %pass% +force_install_dir "%serverpath%" +workshop_download_item 221100 1582671564 validate +quit
move C:\dayz\steamapps\workshop\content\221100\1582671564 C:\dayz\@PartyMe
copy C:\dayz\@PartyMe\Keys\*.bikey C:\dayz\keys

echo "installing VanillaPlusPlusMap"
%steamcmdpath%\steamcmd +login %login% %pass% +force_install_dir "%serverpath%" +workshop_download_item 221100 1623711988 validate +quit
move C:\dayz\steamapps\workshop\content\221100\1623711988 C:\dayz\@VanillaPlusPlusMap
copy C:\dayz\@VanillaPlusPlusMap\Keys\*.bikey C:\dayz\keys

echo "installing Unlimited Stamina"
%steamcmdpath%\steamcmd +login %login% %pass% +force_install_dir "%serverpath%" +workshop_download_item 221100 1560819773 validate +quit
move C:\dayz\steamapps\workshop\content\221100\1560819773 C:\dayz\@UnlimitedStamina
copy C:\dayz\@UnlimitedStamina\Keys\*.bikey C:\dayz\keys

echo "installing Community-Online-Tools"
%steamcmdpath%\steamcmd +login %login% %pass% +force_install_dir "%serverpath%" +workshop_download_item 221100 1564026768 validate +quit
move C:\dayz\steamapps\workshop\content\221100\1564026768 C:\dayz\@Community-Online-Tools
copy C:\dayz\@Community-Online-Tools\Keys\*.bikey C:\dayz\keys

echo "installing Code Lock"
%steamcmdpath%\steamcmd +login %login% %pass% +force_install_dir "%serverpath%" +workshop_download_item 221100 1646187754 validate +quit
move C:\dayz\steamapps\workshop\content\221100\1646187754 C:\dayz\@Code Lock
copy C:\dayz\@CodeLock\Keys\*.bikey C:\dayz\keys

echo "installing BaseBuildingPlus"
%steamcmdpath%\steamcmd +login %login% %pass% +force_install_dir "%serverpath%" +workshop_download_item 221100 1710977250 validate +quit
move C:\dayz\steamapps\workshop\content\221100\1710977250 C:\dayz\@BaseBuildingPlus
copy C:\dayz\@BaseBuildingPlus\Keys\*.bikey C:\dayz\keys

echo "installing BuildAnywhere_v3"
%steamcmdpath%\steamcmd +login %login% %pass% +force_install_dir "%serverpath%" +workshop_download_item 221100 1854626456 validate +quit
move C:\dayz\steamapps\workshop\content\221100\1854626456 C:\dayz\@BuildAnywhere_v3
copy C:\dayz\@BuildAnywhere_v3\Keys\*.bikey C:\dayz\keys

echo "installing Trader"
%steamcmdpath%\steamcmd +login %login% %pass% +force_install_dir "%serverpath%" +workshop_download_item 221100 1590841260 validate +quit
move C:\dayz\steamapps\workshop\content\221100\1590841260 C:\dayz\@Trader
copy C:\dayz\@Trader\Keys\*.bikey C:\dayz\keys

echo "installing Airdrop-Upgraded"
%steamcmdpath%\steamcmd +login %login% %pass% +force_install_dir "%serverpath%" +workshop_download_item 221100 1870524790 validate +quit
move C:\dayz\steamapps\workshop\content\221100\1870524790 C:\dayz\@Airdrop-Upgraded
copy C:\dayz\@Airdrop-Upgraded\Keys\*.bikey C:\dayz\keys

pause



