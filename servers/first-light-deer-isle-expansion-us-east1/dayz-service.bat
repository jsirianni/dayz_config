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
set "MOD_DEERISLE=1750506510"

REM ===== Server @mod folder names (NO SPACES) =====
set "DST_CF=@CF"
set "DST_DEERISLE=@DeerIsle"


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
  +workshop_download_item 221100 %MOD_DEERISLE% validate ^
  +quit
if errorlevel 1 goto :steamfail

REM ===== Sync mods into server @ folders =====
call :syncmod "%MOD_CF%" "%DST_CF%"
REM Deer Isle expansion: do NOT call syncmod for MOD_DEERISLE. The expansion mod
REM does not ship keys in its mod directory; keys are supplied separately from
REM C:\DayZ\deer-isle-exp-keys and copied into %KEYS% in the block below.
REM call :syncmod "%MOD_DEERISLE%" "%DST_DEERISLE%"

REM ===== Copy Deer Isle expansion keys (not in workshop mod) =====
set "DEERISLE_EXP_KEYS=C:\DayZ\deer-isle-exp-keys"
if exist "%DEERISLE_EXP_KEYS%" (
  copy /Y "%DEERISLE_EXP_KEYS%\*" "%KEYS%" >nul
)

REM ===== Build -mod list (RELATIVE paths, NO SPACES in names) =====
set "MODLINE=-mod=@CF;@DeerIsle"


REM ===== Launch DayZ =====
pushd "%INSTALL%"
set "ARGS=-config=%CFG% -port=4801 -profiles=%PROFILES% -dologs -adminlog -netlog -freezecheck -BEpath=C:\DayZ\server\battleye %MODLINE%"
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
