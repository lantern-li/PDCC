VERSION=v2.3.6

build:
	go mod tidy && go build ./...

gomod:
	go get chainmaker.org/chainmaker/chainconf/v2@v2.3.5
	go get chainmaker.org/chainmaker/common/v2@v2.3.8_sz
	go get chainmaker.org/chainmaker/consensus-utils/v2@v2.3.5_sz
	go get chainmaker.org/chainmaker/localconf/v2@v2.3.7_sz
	go get chainmaker.org/chainmaker/logger/v2@v2.3.4_qc
	go get chainmaker.org/chainmaker/lws@v1.2.1
	go get chainmaker.org/chainmaker/pb-go/v2@v2.3.7_sz_determint
	go get chainmaker.org/chainmaker/protocol/v2@v2.3.10_sz_determint
	go get chainmaker.org/chainmaker/utils/v2@v2.3.6_qc_sz
	go mod tidy

lint:
	golangci-lint run ./...