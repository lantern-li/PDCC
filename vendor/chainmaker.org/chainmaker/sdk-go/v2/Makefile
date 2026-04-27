VERSION=v2.3.6

build:
	go mod tidy && go build ./...

lint:
	golangci-lint run ./...

mockgen:
	mockgen -destination ./mock/sdk_mock.go -package mock -source sdk_interface.go

gomod:
	go get chainmaker.org/chainmaker/common/v2@v2.3.8_sz
	go get chainmaker.org/chainmaker/pb-go/v2@v2.3.7_sz_determint
	go get chainmaker.org/chainmaker/protocol/v2@v2.3.10_sz_determint
	go get chainmaker.org/chainmaker/utils/v2@v2.3.6_qc_sz
	go mod tidy

.PHONY: build lint
