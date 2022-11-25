ifeq ($(OS),Windows_NT)
    PLATFORM="Windows"
else
    ifeq ($(shell uname),Darwin)
        PLATFORM="MacOS"
    else
        PLATFORM="Linux"
    endif
endif

VERSION=v2.3.1
DATETIME=$(shell date "+%Y%m%d%H%M%S")
GIT_BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
GIT_COMMIT = $(shell git log --pretty=format:'%h' -n 1)

LOCALCONF_HOME=chainmaker.org/chainmaker-go/module/blockchain
GOLDFLAGS += -X "${LOCALCONF_HOME}.CurrentVersion=${VERSION}"
GOLDFLAGS += -X "${LOCALCONF_HOME}.BuildDateTime=${DATETIME}"
GOLDFLAGS += -X "${LOCALCONF_HOME}.GitBranch=${GIT_BRANCH}"
GOLDFLAGS += -X "${LOCALCONF_HOME}.GitCommit=${GIT_COMMIT}"

BUILD_SERVER=root@192.168.1.5
DEPLOP_1_SERVER=root@192.168.1.5
DEPLOP_2_SERVER=root@192.168.1.6
DEPLOP_3_SERVER=root@192.168.1.7
DEPLOP_4_SERVER=root@192.168.1.8

chainmaker:
    ifeq ($(PLATFORM),"Windows")
		@echo "build for windows"
		@rm -rf go.sum && cd main && go mod tidy && go build -ldflags '${GOLDFLAGS}' -o ../bin/chainmaker.exe
    else
		@echo "build for linux or mac"
		@rm -rf go.sum && cd main && go mod tidy && go build -ldflags '${GOLDFLAGS}' -o ../bin/chainmaker
    endif

autodeploy: vtar-scp deploy-4-node-binary

# 1.vendor ; 2.tar ; 3.scp ; 4.build ;
vtar-scp: gen-clib-vendor tar-source-code scp-source-code build-remote

deploy-4-node-binary:
	# stop node1
	@ssh $(DEPLOP_1_SERVER) "cd /home/sz/node1/bin; ./stop.sh;sleep 2"
	# scp node1 ...
	@ssh $(BUILD_SERVER) "scp -r /home/sz/code/chainmaker/chainmaker-go/bin/chainmaker $(DEPLOP_1_SERVER):/home/sz/node1/bin"
	# start node1
	@ssh $(DEPLOP_1_SERVER) "cd /home/sz/node1/bin; ./start.sh"

	# stop node2
	@ssh $(DEPLOP_2_SERVER) "cd /home/sz/node2/bin; ./stop.sh;sleep 2"
	# scp node2 ...
	@ssh $(BUILD_SERVER) "scp -r /home/sz/code/chainmaker/chainmaker-go/bin/chainmaker $(DEPLOP_2_SERVER):/home/sz/node2/bin"
	# start node2
	@ssh $(DEPLOP_2_SERVER) "cd /home/sz/node2/bin; ./start.sh"

	# stop node3
	@ssh $(DEPLOP_3_SERVER) "cd /home/sz/node3/bin; ./stop.sh;sleep 2"
	# scp node3 ...
	@ssh $(BUILD_SERVER) "scp -r /home/sz/code/chainmaker/chainmaker-go/bin/chainmaker $(DEPLOP_3_SERVER):/home/sz/node3/bin"
	# start node3
	@ssh $(DEPLOP_3_SERVER) "cd /home/sz/node3/bin; ./start.sh"

	# stop node4
	@ssh $(DEPLOP_4_SERVER) "cd /home/sz/node4/bin; ./stop.sh;sleep 2"
	# scp node4 ...
	@ssh $(BUILD_SERVER) "scp -r /home/sz/code/chainmaker/chainmaker-go/bin/chainmaker $(DEPLOP_4_SERVER):/home/sz/node4/bin"
	# start node4
	@ssh $(DEPLOP_4_SERVER) "cd /home/sz/node4/bin; ./start.sh"


build-remote:
	#删除历史源码
	@ssh $(BUILD_SERVER) "cd /home/sz/code/chainmaker; rm -rf chainmaker-go"
	#更新源代码
	@ssh $(BUILD_SERVER) "cd /home/sz/code/chainmaker; tar -xf chainmaker-go.tar.gz"
	#编译
	@ssh $(BUILD_SERVER) "source /etc/profile; cd /home/sz/code/chainmaker/chainmaker-go; make vendor-build"
	#编译编译完成

tar-source-code:
	@cd .. ; tar -czvf chainmaker-go.tar.gz --exclude=chainmaker-go/.git  --exclude=chainmaker-go/test  --exclude=chainmaker-go/bin  --exclude=chainmaker-go/build  --exclude=chainmaker-go/data  --exclude=chainmaker-go/tools/cmc1  --exclude=chainmaker-go/log chainmaker-go

scp-source-code:
	@cd .. ; scp -r chainmaker-go.tar.gz root@192.168.1.1:/home/sz/code/chainmaker
	@cd .. ; scp -r chainmaker-go.tar.gz $(BUILD_SERVER):/home/sz/code/chainmaker
	@cd .. ; scp -r chainmaker-go.tar.gz root@192.168.1.9:/home/sz/code/chainmaker

vendor-build:
	@cd main && go build -mod=vendor -ldflags '${GOLDFLAGS}' -o ../bin/chainmaker

gen-clib-vendor:
	@sudo -S rm -rf vendor
	@go mod vendor
	# 注意：执行此方法前需要切换common项目到对应分支或commit
	# 密码学 gmssl 相关
	@cp -a ../common/opencrypto/gmssl/gmssl/include ./vendor/chainmaker.org/chainmaker/common/v2/opencrypto/gmssl/gmssl/
	@cp -a ../common/opencrypto/gmssl/gmssl/lib ./vendor/chainmaker.org/chainmaker/common/v2/opencrypto/gmssl/gmssl/lib/
	# 密码学 tencentsm 相关
	@cp -a ../common/opencrypto/tencentsm/tencentsm/include ./vendor/chainmaker.org/chainmaker/common/v2/opencrypto/tencentsm/tencentsm/
	@cp -a ../common/opencrypto/tencentsm/tencentsm/lib ./vendor/chainmaker.org/chainmaker/common/v2/opencrypto/tencentsm/tencentsm/
	# 密码学 bulletproofs 相关
	@cp -a ../common/crypto/bulletproofs/bulletproofs_cgo/c_include ./vendor/chainmaker.org/chainmaker/common/v2/crypto/bulletproofs/bulletproofs_cgo/c_include/
	@cp -a ../common/crypto/bulletproofs/bulletproofs_cgo/c_lib ./vendor/chainmaker.org/chainmaker/common/v2/crypto/bulletproofs/bulletproofs_cgo/c_lib/
	# 虚拟机 wasmer-go 相关
	@mkdir -p ./vendor/chainmaker.org/chainmaker/vm-wasmer/v2/wasmer-go/
	@cp -a ${GOPATH}/pkg/mod/chainmaker.org/chainmaker/vm-wasmer/v2@v2.3.1/wasmer-go ./vendor/chainmaker.org/chainmaker/vm-wasmer/v2/wasmer-go/
#	@cp -a ../common/crypto/bulletproofs/bulletproofs_cgo/c_lib ./vendor/chainmaker.org/chainmaker/common/v2/crypto/bulletproofs/bulletproofs_cgo
#	@cp -a ../chainmaker/common/crypto/bulletproofs/bulletproofs_cgo/c_lib/libbulletproofs.a /usr/lib/

package:
	@cd main && go mod tidy && GOPATH=${GOPATH} go build -ldflags '${GOLDFLAGS}' -o ../bin/chainmaker
	@mkdir -p ./release
	@rm -rf ./tmp/chainmaker/
	@mkdir -p ./tmp/chainmaker/
	@mkdir ./tmp/chainmaker/bin
	@mkdir ./tmp/chainmaker/config
	@mkdir ./tmp/chainmaker/log
	@cp bin/chainmaker ./tmp/chainmaker/bin
	@cp -r config ./tmp/chainmaker/
	@cd ./tmp;tar -zcvf chainmaker-$(VERSION).$(DATETIME).$(PLATFORM).tar.gz chainmaker; mv chainmaker-$(VERSION).$(DATETIME).$(PLATFORM).tar.gz ../release
	@rm -rf ./tmp/

compile:
	@cd main && go mod tidy && go build -ldflags '${GOLDFLAGS}' -o ../bin/chainmaker

cmc:
	@cd tools/cmc && go mod tidy && go build -ldflags '${GOLDFLAGS}' -o ../../bin/cmc

send-tool:
	cd test/send_proposal_request_tool && go build -o ../../bin/send_proposal_request_tool

scanner:
	@cd tools/scanner && GOPATH=${GOPATH} go build -o ../../bin/scanner

dep:
	@go get golang.org/x/tools/cmd/stringer

generate:
	go generate ./...

docker-build:
	rm -rf build/ data/ log/ bin/
	docker build -t chainmaker -f ./DOCKER/Dockerfile .
	docker tag chainmaker chainmaker:${VERSION}

docker-build-dev: chainmaker
	docker build -t chainmaker -f ./DOCKER/dev.Dockerfile .
	docker tag chainmaker chainmaker:${VERSION}

docker-compose-start: docker-compose-stop
	docker-compose up -d

docker-compose-stop:
	docker-compose down

ut:
	cd scripts && ./ut_cover.sh

lint:
	cd main && golangci-lint run ./...
	cd module/accesscontrol && golangci-lint run .
	cd module/blockchain && golangci-lint run .
	cd module/core && golangci-lint run ./...
	cd module/consensus && golangci-lint run ./...
	cd module/net && golangci-lint run ./...
	cd module/rpcserver && golangci-lint run ./...
	cd module/snapshot && golangci-lint run ./...
	cd module/subscriber && golangci-lint run ./...
	cd module/sync && golangci-lint run ./...
	cd module/txfilter && golangci-lint run ./...
	golangci-lint run ./tools/cmc/...
	cd tools/scanner && golangci-lint run ./...

gomod:
	cd scripts && ./gomod_update.sh

test-deploy:
	cd scripts/test/ && ./quick_deploy.sh

sql-qta:
	echo "clear environment"
	cd test/chain2 && ./stop.sh
	cd test/chain2 && ./clean.sh
	echo "start new sql-qta test"
	cd test/chain2 && ./build.sh
	cd test/chain2 && ./start.sh
	cd test/scenario0_native && python3 chain2.py
	cd test/scenario1_evm && python3 chain2.py
	cd test/scenario2_rust && python3 chain2.py
	cd test/scenario4_wasmer_sql && python3 chain2.py
	cd test/chain2 && ./stop.sh
	cd test/chain2 && ./clean.sh

qta: cert-qta pub-qta docker-qta

cert-qta:
	echo "clear environment"
	cd test/chain1 && ./stop.sh
	cd test/chain1 && ./clean.sh
	echo "start new qta test"
	cd test/chain1 && ./build.sh
	cd test/chain1 && ./start.sh
	cd test/scenario0_native && python3 chain1.py
	cd test/scenario1_evm && python3 chain1.py
	cd test/scenario2_rust && python3 chain1.py
	cd test/chain1 && ./stop.sh
	cd test/chain1 && ./clean.sh

pub-qta:
	echo "clear environment"
	cd test/chain3 && ./stop.sh
	cd test/chain3 && ./clean.sh
	echo "start new qta test"
	cd test/chain3 && ./build.sh
	cd test/chain3 && ./start.sh
	cd test/scenario0_native && python3 chain3.py
	cd test/scenario1_evm && python3 chain3.py
	#cd test/scenario2_rust && python3 chain3.py  #Rust合约不能启用Gas
	cd test/chain3 && ./stop.sh
	cd test/chain3 && ./clean.sh

docker-qta:
	echo "clear environment"
	cd test/chain1 && ./stop.sh
	cd test/chain1 && ./clean.sh
	echo "start new docker-qta test"
	cd scripts/docker && ./build-dockergo.sh
	cd test/chain1 && ./build.sh
	cd test/chain1 && ./docker-start.sh
	cd test/chain1 && ./start.sh
	cd test/scenario3_dockergo && python3 chain1.py
	cd test/chain1 && ./stop.sh
	cd test/chain1 && ./clean.sh
	docker rm -f  `docker ps -aq -f name=ci-chain1`