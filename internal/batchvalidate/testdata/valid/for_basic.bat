@echo off
for %%A in (*.txt) do echo %%A
for /l %%I in (1,1,3) do echo %%I
for /f "tokens=1,2 delims=," %%A in (input.txt) do echo %%A %%B
