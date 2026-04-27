VERSION=v2.3.8

build:
	go mod tidy && go build ./...

gomod:
	go get chainmaker.org/chainmaker/common/v2@v2.3.8_sz
	go get chainmaker.org/chainmaker/pb-go/v2@v2.3.8_sz
	go get chainmaker.org/chainmaker/protocol/v2@v2.3.10_sz
	go get chainmaker.org/chainmaker/store-badgerdb/v2@v2.3.4
	go get chainmaker.org/chainmaker/store-leveldb/v2@v2.3.4
	go get chainmaker.org/chainmaker/store-sqldb/v2@v2.3.4
	go get chainmaker.org/chainmaker/store-tikv/v2@v2.3.4
	go get chainmaker.org/chainmaker/utils/v2@v2.3.6_qc_sz
	go get chainmaker.org/chainmaker/lws@v1.2.1
	go mod tidy

ut:
	go test ./...
	(gocloc --include-lang=Go --output-type=json . | jq '(.total.comment-.total.files*6)/(.total.code+.total.comment)*100')

mockgen:
	mockgen -destination ./statedb/statedb_mock.go -package statedb -source ./statedb/statedb.go
	mockgen -destination ./blockdb/blockdb_mock.go -package blockdb -source ./blockdb/blockdb.go
	mockgen -destination ./resultdb/resultdb_mock.go -package resultdb -source ./resultdb/resultdb.go

lint:
	golangci-lint run ./...