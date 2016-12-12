cls
git rev-parse HEAD> git-rev.txt
gitrev.exe
rem html2go.exe
rem go install -tags "debug" template
go install commands
go install constant
go install util
go install backend/...  
go install frontend/...  
rem go build -race -o AiDps_v%1.exe main 
rem go build -o 防入侵处理系统_v%1.exe main 
go build DataTrace
rem go build -ldflags -H=windowsgui  datatrace/bak2