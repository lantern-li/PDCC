VERSION=v2.3.5

build:
	go mod tidy && go build ./...

gomod:
	go get chainmaker.org/chainmaker/logger/v2@v2.3.4
	go get chainmaker.org/chainmaker/protocol/v2@${VERSION}
	go mod tidy
