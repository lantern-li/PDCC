module chainmaker.org/chainmaker-go

go 1.16

require (
	chainmaker.org/chainmaker/chainconf/v2 v2.3.1-0.20221109030152-29b09937bc1f
	chainmaker.org/chainmaker/common/v2 v2.3.1-0.20221103080328-39ebe8c6722f
	chainmaker.org/chainmaker/consensus-dpos/v2 v2.3.0
	chainmaker.org/chainmaker/consensus-maxbft/v2 v2.3.0
	chainmaker.org/chainmaker/consensus-raft/v2 v2.3.0
	chainmaker.org/chainmaker/consensus-solo/v2 v2.3.0
	chainmaker.org/chainmaker/consensus-tbft/v2 v2.3.2-0.20221207034953-858325675df2
	chainmaker.org/chainmaker/consensus-utils/v2 v2.3.1
	chainmaker.org/chainmaker/localconf/v2 v2.3.1-0.20221124074151-0b6a3a441c57 // v2.3.0_qc_sz
	chainmaker.org/chainmaker/logger/v2 v2.3.0
	chainmaker.org/chainmaker/net-common v1.2.1
	chainmaker.org/chainmaker/net-libp2p v1.2.2-0.20221210132214-3698e036d0c2
	chainmaker.org/chainmaker/net-liquid v1.1.0
	chainmaker.org/chainmaker/pb-go/v2 v2.3.2-0.20221122133546-6c0d11394fd9
	chainmaker.org/chainmaker/protocol/v2 v2.3.2-0.20221210062113-6dbeaa53eb9c // v2.3.1_qc_sz_store
	chainmaker.org/chainmaker/sdk-go/v2 v2.3.2-0.20221122142750-dcc4c69646dc
	chainmaker.org/chainmaker/store/v2 v2.3.3-0.20221211094956-da7505245f9c // v2.3.1_qc_fix
	chainmaker.org/chainmaker/txpool-batch/v2 v2.3.1-0.20221124033619-e2e42ea261ce
	chainmaker.org/chainmaker/txpool-normal/v2 v2.3.1-0.20221103095550-4fad686735d0
	chainmaker.org/chainmaker/txpool-single/v2 v2.3.1-0.20221103125525-c11ec6726025
	chainmaker.org/chainmaker/utils/v2 v2.3.2-0.20221102022922-e1b8d8236d78
	chainmaker.org/chainmaker/vm-docker-go/v2 v2.3.1-0.20220923021934-5c61122967ac
	chainmaker.org/chainmaker/vm-engine/v2 v2.3.2-0.20221028065733-e2d0d302fb78
	chainmaker.org/chainmaker/vm-evm/v2 v2.3.2-0.20221101040731-6542fc4bae7f
	chainmaker.org/chainmaker/vm-gasm/v2 v2.3.2-0.20221028065828-e6a027d54d7c
	chainmaker.org/chainmaker/vm-native/v2 v2.3.2-0.20221123100205-237e3f956864
	chainmaker.org/chainmaker/vm-wasmer/v2 v2.3.2-0.20221121061121-d88f996f1669
	chainmaker.org/chainmaker/vm-wxvm/v2 v2.3.2-0.20221027124616-c0fb426ec231
	chainmaker.org/chainmaker/vm/v2 v2.3.2-0.20221123035732-3b6078228c84
	code.cloudfoundry.org/bytefmt v0.0.0-20211005130812-5bb3c17173e5
	github.com/Rican7/retry v0.1.0
	github.com/Workiva/go-datastructures v1.0.53
	github.com/c-bata/go-prompt v0.2.2
	github.com/common-nighthawk/go-figure v0.0.0-20210622060536-734e95fb86be
	github.com/gogo/protobuf v1.3.2
	github.com/golang/mock v1.6.0
	github.com/google/shlex v0.0.0-20181106134648-c34317bd91bf
	github.com/gosuri/uiprogress v0.0.1
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/hokaccha/go-prettyjson v0.0.0-20201222001619-a42f9ac2ec8e
	github.com/holiman/uint256 v1.2.0
	github.com/hpcloud/tail v1.0.0
	github.com/linvon/cuckoo-filter v0.4.0
	github.com/mitchellh/mapstructure v1.5.0
	github.com/mr-tron/base58 v1.2.0
	github.com/panjf2000/ants/v2 v2.4.8
	github.com/prometheus/client_golang v1.11.0
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.10.1
	github.com/stretchr/testify v1.7.1
	github.com/syndtr/goleveldb v1.0.1-0.20210305035536-64b5b1c73954
	github.com/tidwall/pretty v1.2.0
	github.com/tmc/grpc-websocket-proxy v0.0.0-20201229170055-e5319fda7802
	go.uber.org/atomic v1.7.0
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f
	golang.org/x/time v0.0.0-20210608053304-ed9ce3a009e4
	google.golang.org/grpc v1.47.0
	gorm.io/driver/mysql v1.2.0
	gorm.io/gorm v1.22.3
)

require (
	github.com/go-echarts/go-echarts/v2 v2.2.4
	github.com/gosuri/uilive v0.0.4 // indirect
	github.com/hashicorp/golang-lru v0.5.5-0.20210104140557-80c98217689d // indirect
	github.com/huin/goupnp v1.0.1-0.20210310174557-0ca763054c88 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/mattn/go-tty v0.0.0-20180907095812-13ff1204f104 // indirect
	github.com/pkg/errors v0.9.1
	github.com/pkg/term v0.0.0-20180730021639-bffc007b7fd5 // indirect
	github.com/tjfoc/gmsm v1.4.1
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
)

replace (
	github.com/RedisBloom/redisbloom-go => chainmaker.org/third_party/redisbloom-go v1.0.0
	github.com/dgraph-io/badger/v3 => chainmaker.org/third_party/badger/v3 v3.0.0
	github.com/libp2p/go-conn-security-multistream v0.2.0 => chainmaker.org/third_party/go-conn-security-multistream v1.0.2
	github.com/libp2p/go-libp2p-core => chainmaker.org/chainmaker/libp2p-core v1.0.0
	github.com/linvon/cuckoo-filter => chainmaker.org/third_party/cuckoo-filter v1.0.0
	github.com/lucas-clemente/quic-go v0.26.0 => chainmaker.org/third_party/quic-go v1.0.0
	github.com/marten-seemann/qtls-go1-15 => chainmaker.org/third_party/qtls-go1-15 v1.0.0
	github.com/marten-seemann/qtls-go1-16 => chainmaker.org/third_party/qtls-go1-16 v1.0.0
	github.com/marten-seemann/qtls-go1-17 => chainmaker.org/third_party/qtls-go1-17 v1.0.0
	github.com/marten-seemann/qtls-go1-18 => chainmaker.org/third_party/qtls-go1-18 v1.0.0
	github.com/syndtr/goleveldb => chainmaker.org/third_party/goleveldb v1.1.0
	github.com/tikv/client-go => chainmaker.org/third_party/tikv-client-go v1.0.0
// google.golang.org/grpc => google.golang.org/grpc v1.26.0
)
