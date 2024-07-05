ifeq ($(OS),Windows_NT)
    PLATFORM="Windows"
else
    ifeq ($(shell uname),Darwin)
        PLATFORM="MacOS"
    else
        PLATFORM="Linux"
    endif
endif

VERSION=v2.3.1.1
DATETIME=$(shell date "+%Y%m%d%H%M%S")
GIT_BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
GIT_COMMIT = $(shell git log --pretty=format:'%h' -n 1)

LOCALCONF_HOME=chainmaker.org/chainmaker-go/module/blockchain
GOLDFLAGS += -X "${LOCALCONF_HOME}.CurrentVersion=${VERSION}"
GOLDFLAGS += -X "${LOCALCONF_HOME}.BuildDateTime=${DATETIME}"
GOLDFLAGS += -X "${LOCALCONF_HOME}.GitBranch=${GIT_BRANCH}"
GOLDFLAGS += -X "${LOCALCONF_HOME}.GitCommit=${GIT_COMMIT}"

# 注意：需要添加ssh互信才能使用以下部署功能
## 编译服务器代码路径（需要提前创建）
#BUILD_DIR=/home/sz/code/chainmaker
## 部署服务器目标路径（需要提前创建）
#DEPLOY_DIR=/home/sz
#BUILD_SERVER=root@127.0.0.1
#DEPLOY_1_SERVER=root@127.0.0.1
#DEPLOY_2_SERVER=root@127.0.0.1
#DEPLOY_3_SERVER=root@127.0.0.1
#DEPLOY_4_SERVER=root@127.0.0.1

# 最小版本4567
## 编译服务器代码路径（需要提前创建）
BUILD_DIR=/home/sz/code/chainmaker
## 部署服务器目标路径（需要提前创建）
DEPLOY_DIR=/home/sz
BUILD_SERVER=root@192.168.1.5
YACE_SERVER=root@192.168.1.9
DEPLOY_1_SERVER=root@192.168.1.5
DEPLOY_2_SERVER=root@192.168.1.6
DEPLOY_3_SERVER=root@192.168.1.7
DEPLOY_4_SERVER=root@192.168.1.8
# 最小版本
## 编译服务器代码路径（需要提前创建）
#BUILD_DIR=/home/sz/code/chainmaker
## 部署服务器目标路径（需要提前创建）
#DEPLOY_DIR=/home/sz
#BUILD_SERVER=root@192.168.1.1
#DEPLOY_1_SERVER=root@192.168.1.1
#DEPLOY_2_SERVER=root@192.168.1.1
#DEPLOY_3_SERVER=root@192.168.1.1
#DEPLOY_4_SERVER=root@192.168.1.1
## 编译服务器代码路径（需要提前创建）
#BUILD_DIR=/home/sz/code/chainmaker
## 部署服务器目标路径（需要提前创建）
#DEPLOY_DIR=/home/sz
#BUILD_SERVER=root@192.168.1.3
#DEPLOY_1_SERVER=root@192.168.1.3
#DEPLOY_2_SERVER=root@192.168.1.3
#DEPLOY_3_SERVER=root@192.168.1.3
#DEPLOY_4_SERVER=root@192.168.1.3
## 编译服务器代码路径（需要提前创建）
#BUILD_DIR=/home/sz/code/chainmaker
## 部署服务器目标路径（需要提前创建）
#DEPLOY_DIR=/home/sz
#BUILD_SERVER=root@192.168.1.4
#DEPLOY_1_SERVER=root@192.168.1.4
#DEPLOY_2_SERVER=root@192.168.1.4
#DEPLOY_3_SERVER=root@192.168.1.4
#DEPLOY_4_SERVER=root@192.168.1.4

# 北京环境
## 编译服务器代码路径（需要提前创建）
#BUILD_DIR=/root/guoxin/v2.3.0_qc_txassign_sz
## 部署服务器目标路径（需要提前创建）
#DEPLOY_DIR=/root/guoxin/v2.3.0_qc_txassign_sz/signle-org
#BUILD_SERVER=root@172.16.12.98
#DEPLOY_1_SERVER=root@172.16.12.98
#DEPLOY_2_SERVER=root@172.16.12.98
#DEPLOY_3_SERVER=root@172.16.12.98
#DEPLOY_4_SERVER=root@172.16.12.98

chainmaker:
    ifeq ($(PLATFORM),"Windows")
		@echo "build for windows"
		@rm -rf go.sum && cd main && go mod tidy && go build -ldflags '${GOLDFLAGS}' -o ../bin/chainmaker.exe
    else
		@echo "build for linux or mac"
		@rm -rf go.sum && cd main && go mod tidy && go build -ldflags '${GOLDFLAGS}' -o ../bin/chainmaker
    endif

autodeploy: gv-pusc-br deploy-4-node-binary

# 1.generate vendor and clib ; 2.package source code  ; 3.scp ; 4.build ;
gv-pusc-br: generate-commit-id generate-vendor package-source-code upload-source-code build-remote

generate-commit-id:
	@echo `git log -1 | awk 'NR==1'` > commit_id

deploy-4-node:
	@ssh $(BUILD_SERVER) "cd $(BUILD_DIR)/chainmaker-go/signle-org; scp -r node1 $(DEPLOY_1_SERVER):$(DEPLOY_DIR)"
	@ssh $(BUILD_SERVER) "cd $(BUILD_DIR)/chainmaker-go/signle-org; scp -r node2 $(DEPLOY_2_SERVER):$(DEPLOY_DIR)"
	@ssh $(BUILD_SERVER) "cd $(BUILD_DIR)/chainmaker-go/signle-org; scp -r node3 $(DEPLOY_3_SERVER):$(DEPLOY_DIR)"
	@ssh $(BUILD_SERVER) "cd $(BUILD_DIR)/chainmaker-go/signle-org; scp -r node4 $(DEPLOY_4_SERVER):$(DEPLOY_DIR)"

deploy-4-node-binary:
	#编译
	@ssh $(BUILD_SERVER) "source /etc/profile; cd $(BUILD_DIR)/chainmaker-go; make vendor-build"
	#编译编译完成

	# stop node1
	@ssh $(DEPLOY_1_SERVER) "cd $(DEPLOY_DIR)/node1/bin; ./stop.sh;sleep 2"
	# scp node1 ...
	@ssh $(BUILD_SERVER) "scp -r $(BUILD_DIR)/chainmaker-go/bin/chainmaker $(DEPLOY_1_SERVER):$(DEPLOY_DIR)/node1/bin"
	# start node1
	@ssh $(DEPLOY_1_SERVER) "cd $(DEPLOY_DIR)/node1/bin; ./start.sh"

	# stop node2
	@ssh $(DEPLOY_2_SERVER) "cd $(DEPLOY_DIR)/node2/bin; ./stop.sh;sleep 2"
	# scp node2 ...
	@ssh $(BUILD_SERVER) "scp -r $(BUILD_DIR)/chainmaker-go/bin/chainmaker $(DEPLOY_2_SERVER):$(DEPLOY_DIR)/node2/bin"
	# start node2
	@ssh $(DEPLOY_2_SERVER) "cd $(DEPLOY_DIR)/node2/bin; ./start.sh"

	# stop node3
	@ssh $(DEPLOY_3_SERVER) "cd $(DEPLOY_DIR)/node3/bin; ./stop.sh;sleep 2"
	# scp node3 ...
	@ssh $(BUILD_SERVER) "scp -r $(BUILD_DIR)/chainmaker-go/bin/chainmaker $(DEPLOY_3_SERVER):$(DEPLOY_DIR)/node3/bin"
	# start node3
	@ssh $(DEPLOY_3_SERVER) "cd $(DEPLOY_DIR)/node3/bin; ./start.sh"

	# stop node4
	@ssh $(DEPLOY_4_SERVER) "cd $(DEPLOY_DIR)/node4/bin; ./stop.sh;sleep 2"
	# scp node4 ...
	@ssh $(BUILD_SERVER) "scp -r $(BUILD_DIR)/chainmaker-go/bin/chainmaker $(DEPLOY_4_SERVER):$(DEPLOY_DIR)/node4/bin"
	# start node4
	@ssh $(DEPLOY_4_SERVER) "cd $(DEPLOY_DIR)/node4/bin; ./start.sh"

build-remote:
	#删除历史源码
	@ssh $(BUILD_SERVER) "cd $(BUILD_DIR); rm -rf chainmaker-go"
	#更新源代码
	@ssh $(BUILD_SERVER) "cd $(BUILD_DIR); tar -xf chainmaker-go.tar.gz"
	#删除历史源码
	@ssh $(YACE_SERVER) "cd $(BUILD_DIR); rm -rf chainmaker-go"
	#更新源代码
	@ssh $(YACE_SERVER) "cd $(BUILD_DIR); tar -xf chainmaker-go.tar.gz"

package-source-code:
	@cd .. ; tar -czvf chainmaker-go-source.tar.gz --exclude=chainmaker-go/.git  --exclude=chainmaker-go/test  --exclude=chainmaker-go/bin  --exclude=chainmaker-go/build  --exclude=chainmaker-go/data  --exclude=chainmaker-go/tools/cmc1  --exclude=chainmaker-go/log --exclude=.DS_Store chainmaker-go

upload-source-code:
	@cd .. ; scp -r chainmaker-go.tar.gz $(BUILD_SERVER):$(BUILD_DIR)
	@cd .. ; scp -r chainmaker-go.tar.gz $(YACE_SERVER):$(BUILD_DIR)

vendor-build:
	#ln -s vendor/chainmaker.org/chainmaker/vm-wasmer/v2/wasmer-go/wasmer-go/packaged/lib/linux-aarch64/libwasmer.so /usr/lib
	@cd main && go build -mod=vendor -ldflags '${GOLDFLAGS}' -o ../bin/chainmaker

generate-vendor:
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
	@cp -a ${GOPATH}/pkg/mod/chainmaker.org/chainmaker/vm-wasmer/v2@v2.3.2-0.20221121061121-d88f996f1669/wasmer-go ./vendor/chainmaker.org/chainmaker/vm-wasmer/v2/wasmer-go/
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
