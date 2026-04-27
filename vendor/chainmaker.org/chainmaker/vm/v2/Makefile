VERSION=v2.3.7_sz

build:
	go mod tidy && go build ./...

ut:
	mkdir -p ../ut
	go test -covermode=count -coverprofile=../ut/vm.out ./...
	go tool cover -html=../ut/vm.out -o=../ut/vm.html

lint:
	golangci-lint run ./...

gomod:
	go get chainmaker.org/chainmaker/pb-go/v2@v2.3.7_sz_determint
	go get chainmaker.org/chainmaker/common/v2@v2.3.8_sz
	go get chainmaker.org/chainmaker/protocol/v2@v2.3.10_sz_determint
	go get chainmaker.org/chainmaker/utils/v2@v2.3.6_qc_sz
	go get chainmaker.org/chainmaker/chainconf/v2@v2.3.5
	go get chainmaker.org/chainmaker/vm-native/v2@v2.3.8_sz_determint
	go get chainmaker.org/chainmaker/logger/v2@v2.3.4
	go mod tidy
