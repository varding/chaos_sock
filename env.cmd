set PROMPT=$g$s
set GOPATH=%~dp0
set GOBIN=%GOPATH%bin\windows_amd64
set PATH=%PATH%;%GOBIN%
%~d0
cls
rem start console\Console2\Console.exe
start cmd.exe