@echo off
setlocal enabledelayedexpansion
set SERVICE=DayZServer
if defined SERVICE (
  echo Starting !SERVICE!
)
endlocal
