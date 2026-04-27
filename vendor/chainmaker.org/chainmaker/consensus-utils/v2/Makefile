VERSION=v2.3.7_sz

build:
	go mod tidy && go build ./...

gomod:
	go get chainmaker.org/chainmaker/common/v2@v2.3.8_sz
	go get chainmaker.org/chainmaker/localconf/v2@$(VERSION)
	go get chainmaker.org/chainmaker/logger/v2@v2.3.4_qc
	go get chainmaker.org/chainmaker/net-common@v1.2.8_sz
	go get chainmaker.org/chainmaker/net-libp2p@v1.2.11_sz
	go get chainmaker.org/chainmaker/net-liquid@v1.2.1_qc
	go get chainmaker.org/chainmaker/pb-go/v2@v2.3.8_sz
	go get chainmaker.org/chainmaker/protocol/v2@v2.3.10_sz
	go get chainmaker.org/chainmaker/utils/v2@v2.3.6_qc_sz
	go mod tidy

lint:
	golangci-lint run ./...
