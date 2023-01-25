DST     := .
BIN     := imgMerge
GOARCH  := amd64
VERSION := 0.1

FLAGS_VERSION=-X main.version=$(VERSION) -X main.revision=$(git rev-parse --short HEAD)
FLAGS=-tags netgo -installsuffix netgo -trimpath "-ldflags=-buildid=" -ldflags '-s -w -extldflags "-static"'
#FLAGS_WIN=-tags netgo -installsuffix netgo -trimpath "-ldflags=-buildid=" -ldflags '-s -w -extldflags "-static" -H windowsgui'
# コマンドプロンプト出したほうが分かりやすいので出す
FLAGS_WIN=-tags netgo -installsuffix netgo -trimpath "-ldflags=-buildid=" -ldflags '-s -w -extldflags "-static"'


all:
	cp -rf var.go.false.txt var.go
	make a
	cp -rf var.go.true.txt var.go
	make a BIN=imgMerge_reverse

a:
	make win
	make linux

win:
	#GOARCH=$(GOARCH) GOOS=windows go build -o $(DST)/$(BIN)_windows.exe $(FLAGS_WIN) 
	GOARCH=$(GOARCH) GOOS=windows go build -o $(DST)/$(BIN).exe $(FLAGS_WIN) 
	rm -rf $(DST)/$(BIN).upx.exe && upx $(DST)/$(BIN).exe -o $(DST)/$(BIN).upx.exe
	rm -rf $(DST)/$(BIN).exe
	mv $(DST)/$(BIN).upx.exe $(DST)/$(BIN).exe
linux:
	#GOARCH=$(GOARCH) GOOS=linux go build -o $(DST)/$(BIN)_linux $(FLAGS_UNIX) $(FLAGS)
	GOARCH=$(GOARCH) GOOS=linux go build -o $(DST)/$(BIN) $(FLAGS_UNIX) $(FLAGS)
	rm -rf $(DST)/$(BIN).upx && upx $(DST)/$(BIN) -o $(DST)/$(BIN).upx
	rm -rf $(DST)/$(BIN)
	mv $(DST)/$(BIN).upx $(DST)/$(BIN)
