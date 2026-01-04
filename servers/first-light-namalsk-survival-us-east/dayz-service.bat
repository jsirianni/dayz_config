@echo off
setlocal enableextensions

REM ===== Paths =====
set "STEAMCMD=C:\SteamCMD\steamcmd.exe"
set "INSTALL=C:\DayZ\server"
set "SERVEREXE=C:\DayZ\server\DayZServer_x64.exe"
set "PROFILES=C:\DayZ\server\profiles"
set "CFG=C:\DayZ\server\serverDZ.cfg"
set "KEYS=C:\DayZ\server\keys"
set "WSCONTENT=C:\SteamCMD\steamapps\workshop\content\221100"

REM ===== Workshop IDs =====
set "MOD_CF=1559212036"
set "MOD_VPP=1828439124"
set "MOD_NAMALSK=2289456201"
set "MOD_NAMALSK_SURVIVAL=2289461232"
set "MOD_FOG=2931560672"
set "MOD_CODELOCK=1646187754"
set "MOD_VEHICLE3PP=2122332595"
set "MOD_DABS=2545327648"
set "MOD_EXPANSION_LICENSED=2116157322"
set "MOD_EXPANSION_BUNDLE=2572331007"
set "MOD_TERJECORE=3359676785"
set "MOD_TERJEMEDICINE=3359677479"
set "MOD_TERJESKILLS=3359678303"

REM ===== Server @mod folder names (NO SPACES, but dashes are allowed) =====
set "DST_CF=@CF"
set "DST_VPP=@VPPAdminTools"
set "DST_NAMALSK=@NamalskIsland"
set "DST_NAMALSK_SURVIVAL=@NamalskSurvival"
set "DST_FOG=@Fog"
set "DST_CODELOCK=@CodeLock"
set "DST_VEHICLE3PP=@Vehicle3PP"
set "DST_DABS=@DabsFramework"
set "DST_EXPANSION_LICENSED=@DayZ-Expansion-Licensed"
set "DST_EXPANSION_BUNDLE=@DayZ-Expansion-Bundle"
set "DST_TERJECORE=@TerjeCore"
set "DST_TERJEMEDICINE=@TerjeMedicine"
set "DST_TERJESKILLS=@TerjeSkills"

REM ===== Prep =====
if not exist "%INSTALL%"  mkdir "%INSTALL%"
if not exist "%PROFILES%" mkdir "%PROFILES%"
if not exist "%KEYS%"     mkdir "%KEYS%"

REM ===== Update DayZ server =====
"%STEAMCMD%" +force_install_dir "%INSTALL%" +login "stealthowl1" +app_update 223350 validate +quit
if errorlevel 1 goto :steamfail
if not exist "%SERVEREXE%" goto :noexe

REM ===== Update workshop mods =====
"%STEAMCMD%" ^
  +login "stealthowl1" ^
  +workshop_download_item 221100 %MOD_CF% validate ^
  +workshop_download_item 221100 %MOD_VPP% validate ^
  +workshop_download_item 221100 %MOD_NAMALSK% validate ^
  +workshop_download_item 221100 %MOD_NAMALSK_SURVIVAL% validate ^
  +workshop_download_item 221100 %MOD_FOG% validate ^
  +workshop_download_item 221100 %MOD_CODELOCK% validate ^
  +workshop_download_item 221100 %MOD_VEHICLE3PP% validate ^
  +workshop_download_item 221100 %MOD_DABS% validate ^
  +workshop_download_item 221100 %MOD_EXPANSION_LICENSED% validate ^
  +workshop_download_item 221100 %MOD_EXPANSION_BUNDLE% validate ^
  +workshop_download_item 221100 %MOD_TERJECORE% validate ^
  +workshop_download_item 221100 %MOD_TERJEMEDICINE% validate ^
  +workshop_download_item 221100 %MOD_TERJESKILLS% validate ^
  +quit
if errorlevel 1 goto :steamfail

REM ===== Sync mods into server @ folders =====
call :syncmod "%MOD_CF%"       "%DST_CF%"
call :syncmod "%MOD_VPP%"      "%DST_VPP%"
call :syncmod "%MOD_NAMALSK%"  "%DST_NAMALSK%"
call :syncmod "%MOD_NAMALSK_SURVIVAL%" "%DST_NAMALSK_SURVIVAL%"
call :syncmod "%MOD_FOG%" "%DST_FOG%"
call :syncmod "%MOD_CODELOCK%" "%DST_CODELOCK%"
call :syncmod "%MOD_VEHICLE3PP%" "%DST_VEHICLE3PP%"
call :syncmod "%MOD_DABS%"     "%DST_DABS%"
call :syncmod "%MOD_EXPANSION_LICENSED%" "%DST_EXPANSION_LICENSED%"
call :syncmod "%MOD_EXPANSION_BUNDLE%" "%DST_EXPANSION_BUNDLE%"
call :syncmod "%MOD_TERJECORE%" "%DST_TERJECORE%"
call :syncmod "%MOD_TERJEMEDICINE%" "%DST_TERJEMEDICINE%"
call :syncmod "%MOD_TERJESKILLS%" "%DST_TERJESKILLS%"

REM ===== Build -mod list (RELATIVE paths, CF FIRST, NO SPACES in names) =====
REM Expansion mods must load after CF and DabsFramework, Bundle before Licensed
set "MODLINE=-mod=@CF;@VPPAdminTools;@NamalskIsland;@NamalskSurvival;@Fog;@DabsFramework;@DayZ-Expansion-Bundle;@DayZ-Expansion-Licensed;@CodeLock;@Vehicle3PP;@TerjeCore;@TerjeMedicine;@TerjeSkills"

REM ===== Launch DayZ =====
pushd "%INSTALL%"
set "ARGS=-config=%CFG% -port=2601 -profiles=%PROFILES% -dologs -adminlog -netlog -freezecheck -BEpath=C:\DayZ\server\battleye %MODLINE%"
echo Starting DayZ with: %MODLINE%
"%SERVEREXE%" %ARGS%
set "EXITCODE=%ERRORLEVEL%"
popd

echo DayZ exited with code %EXITCODE%
exit /b %EXITCODE%

:syncmod
REM %1 = workshop id, %2 = @dest folder name (no spaces)
set "SRC=%WSCONTENT%\%~1"
set "DST=%INSTALL%\%~2"
if not exist "%SRC%" (
  echo [WARN] Workshop path missing: %SRC%
  goto :eof
)
if not exist "%DST%" mkdir "%DST%"
robocopy "%SRC%" "%DST%" /MIR >nul

REM copy all key files from mod to server\keys (supports multiple key file patterns)
for %%K in ("%DST%\*.bikey") do copy /Y "%%~fK" "%KEYS%" >nul
for %%K in ("%DST%\*.key") do copy /Y "%%~fK" "%KEYS%" >nul
for %%K in ("%DST%\*key*") do copy /Y "%%~fK" "%KEYS%" >nul
if exist "%DST%\keys" (
  for %%K in ("%DST%\keys\*.bikey") do copy /Y "%%~fK" "%KEYS%" >nul
  for %%K in ("%DST%\keys\*.key") do copy /Y "%%~fK" "%KEYS%" >nul
  for %%K in ("%DST%\keys\*key*") do copy /Y "%%~fK" "%KEYS%" >nul
)
goto :eof

:steamfail
echo [ERROR] SteamCMD failed with errorlevel %ERRORLEVEL%
exit /b %ERRORLEVEL%

:noexe
echo [ERROR] Missing "%SERVEREXE%"
exit /b 2
