set PROMPT=$g$s
set GOPATH=%~dp0
rem set GOBIN=%GOPATH%bin\windows_amd64
set PATH=%PATH%;%GOBIN%
%~d0
rem start K:\Sublime3\sublime_text.exe   --project AiDps.sublime-project
start sublime_text.exe   --project chaos_sock.sublime-project
rem cmd /K