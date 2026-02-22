@echo off
setlocal enableextensions enabledelayedexpansion

REM =============================================================================
REM DayZ Server Startup + SteamCMD Update + Workshop Mods Sync (CF + Deer Isle Exp)
REM Fix: ensure Deer Isle workshop content is mirrored into server @DeerIsle folder
REM =============================================================================

REM ===== Paths =====
set "STEAMCMD=C:\SteamCMD\steamcmd.exe"
set "INSTALL=C:\DayZ\server"
set "SERVEREXE=%INSTALL%\DayZServer_x64.exe"
set "PROFILES=%INSTALL%\profiles"
set "CFG=%INSTALL%\serverDZ.cfg"
set "KEYS=%INSTALL%\keys"
set "WSCONTENT=C:\SteamCMD\steamapps\workshop\content\221100"

REM ===== Workshop IDs =====
set "MOD_CF=1559212036"
set "MOD_DEERISLE=1750506510"
set "MOD_VPP=1828439124"
set "MOD_VEHICLE3PP=2122332595"
set "MOD_DABS=2545327648"
set "MOD_EXPANSION_LICENSED=2116157322"
set "MOD_EXPANSION_BUNDLE=2572331007"
set "MOD_TERJECORE=3649957186"
set "MOD_TERJEMEDICINE=3649957536"
set "MOD_TERJESKILLS=3649958397"

REM ===== Server @mod folder names (NO SPACES, dashes allowed) =====
set "DST_CF=@CF"
set "DST_DEERISLE=@DeerIsle"
set "DST_VPP=@VPPAdminTools"
set "DST_VEHICLE3PP=@Vehicle3PP"
set "DST_DABS=@DabsFramework"
set "DST_EXPANSION_LICENSED=@DayZ-Expansion-Licensed"
set "DST_EXPANSION_BUNDLE=@DayZ-Expansion-Bundle"
set "DST_TERJECORE=@TerjeCore"
set "DST_TERJEMEDICINE=@TerjeMedicine"
set "DST_TERJESKILLS=@TerjeSkills"

REM ===== Optional: external key drop for Deer Isle (if you keep keys separately) =====
set "DEERISLE_EXP_KEYS=C:\DayZ\deer-isle-exp-keys"

REM ===== Prep =====
if not exist "%INSTALL%"  mkdir "%INSTALL%"
if not exist "%PROFILES%" mkdir "%PROFILES%"
if not exist "%KEYS%"     mkdir "%KEYS%"

REM ===== Basic sanity =====
if not exist "%STEAMCMD%" (
  echo [ERROR] SteamCMD not found: "%STEAMCMD%"
  exit /b 2
)

REM ===== Update DayZ server =====
echo === Updating DayZ Dedicated Server (app 223350) ===
"%STEAMCMD%" +force_install_dir "%INSTALL%" +login "stealthowl1" +app_update 223350 validate +quit
if errorlevel 1 goto :steamfail

if not exist "%SERVEREXE%" (
  echo [ERROR] Missing "%SERVEREXE%"
  exit /b 2
)

REM ===== Update workshop mods =====
echo === Updating Workshop Mods (app 221100) ===
"%STEAMCMD%" ^
  +login "stealthowl1" ^
  +workshop_download_item 221100 %MOD_CF% validate ^
  +workshop_download_item 221100 %MOD_DEERISLE% validate ^
  +workshop_download_item 221100 %MOD_VPP% validate ^
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
echo === Syncing mods into server folders ===
call :syncmod "%MOD_CF%" "%DST_CF%"
call :syncmod "%MOD_DEERISLE%" "%DST_DEERISLE%"
call :syncmod "%MOD_VPP%" "%DST_VPP%"
call :syncmod "%MOD_VEHICLE3PP%" "%DST_VEHICLE3PP%"
call :syncmod "%MOD_DABS%" "%DST_DABS%"
call :syncmod "%MOD_EXPANSION_LICENSED%" "%DST_EXPANSION_LICENSED%"
call :syncmod "%MOD_EXPANSION_BUNDLE%" "%DST_EXPANSION_BUNDLE%"
call :syncmod "%MOD_TERJECORE%" "%DST_TERJECORE%"
call :syncmod "%MOD_TERJEMEDICINE%" "%DST_TERJEMEDICINE%"
call :syncmod "%MOD_TERJESKILLS%" "%DST_TERJESKILLS%"

REM ===== Copy external Deer Isle keys (optional) =====
if exist "%DEERISLE_EXP_KEYS%" (
  echo === Copying external Deer Isle keys from "%DEERISLE_EXP_KEYS%" ===
  copy /Y "%DEERISLE_EXP_KEYS%\*" "%KEYS%" >nul
)

REM ===== Verify Deer Isle actually staged (world load fix) =====
if not exist "%INSTALL%\%DST_DEERISLE%\addons" (
  echo [ERROR] "%INSTALL%\%DST_DEERISLE%\addons" does not exist.
  echo         Deer Isle mod did not stage correctly; server will NOT find world "deerisle".
  echo         Check workshop download folder: "%WSCONTENT%\%MOD_DEERISLE%"
  exit /b 3
)

REM (optional) quick check for any PBOs
dir /b "%INSTALL%\%DST_DEERISLE%\addons\*.pbo" >nul 2>&1
if errorlevel 1 (
  echo [ERROR] No PBOs found in "%INSTALL%\%DST_DEERISLE%\addons".
  echo         Deer Isle mod did not stage correctly; server will NOT find world "deerisle".
  echo         Check workshop download folder: "%WSCONTENT%\%MOD_DEERISLE%"
  exit /b 3
)

REM ===== Build -mod list (RELATIVE paths; CF first; Dabs before Expansion; Bundle before Licensed) =====
set "MODLINE=-mod=%DST_CF%;%DST_DEERISLE%;%DST_VPP%;%DST_DABS%;%DST_EXPANSION_BUNDLE%;%DST_EXPANSION_LICENSED%;%DST_VEHICLE3PP%;%DST_TERJECORE%;%DST_TERJEMEDICINE%;%DST_TERJESKILLS%"

REM ===== Launch DayZ =====
pushd "%INSTALL%"
set "ARGS=-config=%CFG% -port=4801 -profiles=%PROFILES% -dologs -adminlog -netlog -freezecheck -BEpath=%INSTALL%\battleye %MODLINE%"
echo.
echo Starting DayZ with: %MODLINE%
echo Executable: "%SERVEREXE%"
echo.

"%SERVEREXE%" %ARGS%
set "EXITCODE=%ERRORLEVEL%"
popd

echo DayZ exited with code %EXITCODE%
exit /b %EXITCODE%

REM =============================================================================
REM Functions
REM =============================================================================

:syncmod
REM %1 = workshop id, %2 = @dest folder name
set "SRC=%WSCONTENT%\%~1"
set "DST=%INSTALL%\%~2"

if not exist "%SRC%" (
  echo [ERROR] Workshop path missing: "%SRC%"
  exit /b 4
)

if not exist "%DST%" mkdir "%DST%"

REM Mirror workshop content into server @folder
robocopy "%SRC%" "%DST%" /MIR >nul
if errorlevel 8 (
  echo [ERROR] robocopy failed mirroring "%SRC%" to "%DST%" (errorlevel %ERRORLEVEL%)
  exit /b 5
)

REM Copy key files from mod to server\keys (covers various layouts)
for %%K in ("%DST%\*.bikey") do copy /Y "%%~fK" "%KEYS%" >nul
for %%K in ("%DST%\*.key") do copy /Y "%%~fK" "%KEYS%" >nul
if exist "%DST%\keys" (
  for %%K in ("%DST%\keys\*.bikey") do copy /Y "%%~fK" "%KEYS%" >nul
  for %%K in ("%DST%\keys\*.key") do copy /Y "%%~fK" "%KEYS%" >nul
)
goto :eof

:steamfail
echo [ERROR] SteamCMD failed with errorlevel %ERRORLEVEL%
exit /b %ERRORLEVEL%