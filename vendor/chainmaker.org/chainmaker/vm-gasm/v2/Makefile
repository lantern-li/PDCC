VERSION=v2.3.6

gomod:
	go get chainmaker.org/chainmaker/common/v2@v2.3.8_sz
	go get chainmaker.org/chainmaker/logger/v2@v2.3.4
	go get chainmaker.org/chainmaker/pb-go/v2@v2.3.8_sz
	go get chainmaker.org/chainmaker/protocol/v2@v2.3.10_sz
	go get chainmaker.org/chainmaker/store/v2@v2.3.8_sz
	go get chainmaker.org/chainmaker/vm/v2@v2.3.8_sz
	go mod tidy

lint:
	golangci-lint run ./...

test:
	go test -v ./...