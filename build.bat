@echo off
echo Building PCBox...

echo [1/2] Building frontend...
cd frontend
call pnpm run build
cd ..

echo [2/2] Building binary with wails...

setlocal


set PCBOX_BUILD=1


REN wails build -devtools

wails build  -debug


endlocal


echo Done! Binary at build\bin\pcbox.exe
echo Usage:
echo   pcbox.exe                    - Server mode (default, tray + window)
echo   pcbox.exe --mode=standalone  - Standalone mode (single process)
echo   pcbox.exe --mode=window      - Window mode (connects to server)
echo.
echo Env vars:
echo   PCBOX_DEVTOOLS=1             - Open webview devtools on startup
