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
set "MOD_DEERISLE=1602372402"
set "MOD_CODELOCK=1646187754"
set "MOD_VEHICLE3PP=2122332595"
set "MOD_REDFALCON=2906371600"
set "MOD_CROCODILE=3013430583"
set "MOD_SHARK=3147619641"
set "MOD_DBO_DINO=3096188434"
set "MOD_DINO=3162169329"
set "MOD_FIRESTICKS=3498006019"
set "MOD_4KBOSSK=3369325490"
set "MOD_DOG=2471347750"
set "MOD_YAHT=3354681846"

REM ===== Server @mod folder names (NO SPACES) =====
set "DST_CF=@CF"
set "DST_VPP=@VPPAdminTools"
set "DST_DEERISLE=@DeerIsle"
set "DST_CODELOCK=@CodeLock"
set "DST_VEHICLE3PP=@Vehicle3PP"
set "DST_REDFALCON=@RedFalconBoat"
set "DST_CROCODILE=@Crocodile"
set "DST_SHARK=@Shark"
set "DST_DBO_DINO=@DboDino"
set "DST_DINO=@Dino"
set "DST_FIRESTICKS=@FireSticks"
set "DST_4KBOSSK=@4KBOSSKVehicles"
set "DST_DOG=@Dog"
set "DST_YAHT=@Yaht"

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
  +workshop_download_item 221100 %MOD_DEERISLE% validate ^
  +workshop_download_item 221100 %MOD_CODELOCK% validate ^
  +workshop_download_item 221100 %MOD_VEHICLE3PP% validate ^
  +workshop_download_item 221100 %MOD_REDFALCON% validate ^
  +workshop_download_item 221100 %MOD_CROCODILE% validate ^
  +workshop_download_item 221100 %MOD_SHARK% validate ^
  +workshop_download_item 221100 %MOD_DBO_DINO% validate ^
  +workshop_download_item 221100 %MOD_DINO% validate ^
  +workshop_download_item 221100 %MOD_FIRESTICKS% validate ^
  +workshop_download_item 221100 %MOD_4KBOSSK% validate ^
  +workshop_download_item 221100 %MOD_DOG% validate ^
  +workshop_download_item 221100 %MOD_YAHT% validate ^
  +quit
if errorlevel 1 goto :steamfail

REM ===== Sync mods into server @ folders =====
call :syncmod "%MOD_CF%"       "%DST_CF%"
call :syncmod "%MOD_VPP%"      "%DST_VPP%"
call :syncmod "%MOD_DEERISLE%" "%DST_DEERISLE%"
call :syncmod "%MOD_CODELOCK%" "%DST_CODELOCK%"
call :syncmod "%MOD_VEHICLE3PP%" "%DST_VEHICLE3PP%"
call :syncmod "%MOD_REDFALCON%" "%DST_REDFALCON%"
call :syncmod "%MOD_CROCODILE%" "%DST_CROCODILE%"
call :syncmod "%MOD_SHARK%" "%DST_SHARK%"
call :syncmod "%MOD_DBO_DINO%" "%DST_DBO_DINO%"
call :syncmod "%MOD_DINO%" "%DST_DINO%"
call :syncmod "%MOD_FIRESTICKS%" "%DST_FIRESTICKS%"
call :syncmod "%MOD_4KBOSSK%" "%DST_4KBOSSK%"
call :syncmod "%MOD_DOG%" "%DST_DOG%"
call :syncmod "%MOD_YAHT%" "%DST_YAHT%"

REM ===== Build -mod list (RELATIVE paths, CF FIRST, NO SPACES in names) =====
set "MODLINE=-mod=@CF;@VPPAdminTools;@DeerIsle;@CodeLock;@Vehicle3PP;@RedFalconBoat;@Crocodile;@Shark;@DboDino;@Dino;@FireSticks;@4KBOSSKVehicles;@Dog;@Yaht"

REM ===== Launch DayZ =====
pushd "%INSTALL%"
set "ARGS=-config=%CFG% -port=2801 -profiles=%PROFILES% -dologs -adminlog -netlog -freezecheck -BEpath=C:\DayZ\server\battleye %MODLINE%"
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
