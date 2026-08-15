@echo off
echo output >out.txt
echo error 2>err.txt
sort 0<input.txt
echo merged 2>&1
