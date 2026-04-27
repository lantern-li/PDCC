VERSION=v2.3.5

build:
	go mod tidy && go build ./...

gomod:
	go get chainmaker.org/chainmaker/common/v2@v2.3.4
	go get chainmaker.org/chainmaker/localconf/v2@v2.3.4
	go get chainmaker.org/chainmaker/lws@v1.2.1
	go get chainmaker.org/chainmaker/pb-go/v2@$(VERSION)
	go get chainmaker.org/chainmaker/protocol/v2@$(VERSION)
	go get chainmaker.org/chainmaker/utils/v2@$(VERSION)
	go mod tidy

ut:
	mkdir -p ../ut
	go test -v -coverprofile=../ut/txpool-batch.out ./...
	go tool cover -html=../ut/txpool-batch.out -o ../ut/txpool-batch.html

lint:
	golangci-lint run ./...