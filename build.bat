SET GOOS=linux
SET GOARCH=amd64

go build -o brdraft_linux_amd64 brdraft

SET GOOS=darwin
SET GOARCH=amd64

go build -o brdraft_osx_amd64 brdraft

SET GOOS=windows
SET GOARCH=amd64

go build -o brdraft_windows_amd64.exe brdraft
