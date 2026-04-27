VERSION=v2.3.7
IMAGE_VERSION=v2.3.7

ARCH="`uname -s`"
LINUX="Linux"
Darwin="Darwin"

BUILD_TIME = $(shell date "+%Y%m%d%H%M%S")
GIT_BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
GIT_COMMIT = $(shell git log --pretty=format:'%h' -n 1)

build-test:
	cd test/scripts && ./prepare.sh

build-image:
	cd vm_mgr && go mod vendor
	cd vm_mgr && docker build -t chainmaker-vm-engine \
	--build-arg BUILD_TIME=${BUILD_TIME} \
	--build-arg GIT_BRANCH=${GIT_BRANCH} \
	--build-arg GIT_COMMIT=${GIT_COMMIT} \
	-f Dockerfile ./
	docker tag chainmaker-vm-engine chainmakerofficial/chainmaker-vm-engine:${IMAGE_VERSION}
	docker images | grep chainmaker-vm-engine

build-image-arch:
	cd vm_mgr && go mod vendor
	cd vm_mgr && GOOS=linux $(if $(arch),GOARCH=$(arch),) go build -o ../bin/startvm ./main.go
	docker build -t chainmaker-vm-engine \
	--build-arg BUILD_TIME=${BUILD_TIME} \
	--build-arg GIT_BRANCH=${GIT_BRANCH} \
	--build-arg GIT_COMMIT=${GIT_COMMIT} \
	-f vm_mgr/dev.Dockerfile ./
	docker tag chainmaker-vm-engine chainmakerofficial/chainmaker-vm-engine:${IMAGE_VERSION}
	docker images | grep chainmaker-vm-engine

image-push:
	docker push chainmakerofficial/chainmaker-vm-engine:${IMAGE_VERSION}

update-gomod:
	cd vm_mgr && rm -rf vendor
	bash .vm_mgr/scripts/gomod_update.sh

gen-dockervm-pb:
	cd pb/proto && protoc -I=. --gogofaster_out=plugins=grpc:../protogo --gogofaster_opt=paths=source_relative dockervm_message.proto
	cd vm_mgr/pb/proto && protoc -I=. --gogofaster_out=plugins=grpc:../protogo --gogofaster_opt=paths=source_relative dockervm_message.proto

clean-test:
	./test/scripts/prepare.sh

clean:
	cd vm_mgr && rm -rf vendor
	./test/scripts/prepare.sh

ci:
	golangci-lint run ./...

gomod:
	go get chainmaker.org/chainmaker/common/v2@v2.3.8_sz
	go get chainmaker.org/chainmaker/localconf/v2@v2.3.7_sz
	go get chainmaker.org/chainmaker/pb-go/v2@v2.3.8_sz
	go get chainmaker.org/chainmaker/protocol/v2@v2.3.10_sz
	go get chainmaker.org/chainmaker/utils/v2@v2.3.6_qc_sz
	go get chainmaker.org/chainmaker/vm/v2@v2.3.8_sz
	go mod tidy

ut:
	cd vm_mgr && rm -rf vendor
	./test/scripts/prepare.sh
	make build-image
	docker run -itd --net=host --privileged --name chaimaker_vm_test chainmakerofficial/chainmaker-vm-engine:${IMAGE_VERSION}
	docker stop chaimaker_vm_arch_test >/dev/null 2>&1 || true
	docker rm chaimaker_vm_arch_test >/dev/null 2>&1 || true
	make build-image-arch
	@if [ $(ARCH) = $(LINUX) ];then \
  		docker run -itd -p 21521:21521 -e CHAIN_RPC_PORT="21521" -e SANDBOX_RPC_PORT="23521" --privileged --add-host=host.docker.internal:host-gateway --name chaimaker_vm_arch_test chainmakerofficial/chainmaker-vm-engine:${VERSION}; \
	elif [ $(ARCH) = $(Darwin) ];then \
  		docker run -itd -p 21521:21521 -e CHAIN_RPC_PORT="21521" -e SANDBOX_RPC_PORT="23521" --privileged --name chaimaker_vm_arch_test chainmakerofficial/chainmaker-vm-engine:${VERSION}; \
	elif [ "$(expr substr $(ARCH) 1 5)"=="$(Windows)" ];then \
  		docker run -itd -p 21521:21521 -e CHAIN_RPC_PORT="21521" -e SANDBOX_RPC_PORT="23521" --privileged --add-host=docker.for.win.localhost:host-gateway --name chaimaker_vm_arch_test chainmakerofficial/chainmaker-vm-engine:${VERSION}; \
	else \
	  echo "unknown ARCH"; \
	fi
	sh ./ut_cover.sh

version:
	docker inspect chainmakerofficial/chainmaker-vm-engine:${IMAGE_VERSION} | jq '.[].ContainerConfig.Labels'
