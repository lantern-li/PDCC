package main

import (
	"flag"
	"fmt"
	"math/big"
	"os"

	"github.com/syndtr/goleveldb/leveldb"
)

// eg: -db_path yourdatapath/data/wx-org1.chainmaker.org/state/chain1/store_state -step 1 -user_addr FvgKhDjcmnKi8TzKGCiXFt6umiBysY36xKuCYQrYmk2e -contract_name DPOS_ERC20

func main() {
	var (
		step    int
		dbPath  string
		accAddr string
		contrnm string
	)
	flag.IntVar(&step, "step", 0, "0: modify amount; 1: recovery amount")
	flag.StringVar(&dbPath, "db_path", "", "db of the path")
	flag.StringVar(&accAddr, "user_addr", "", "acc of the addr")
	flag.StringVar(&contrnm, "contract_name", "", "name of the contract")
	flag.Parse()
	if len(dbPath) == 0 || len(accAddr) == 0 {
		panic(fmt.Sprintf("db_path[%s] or user_addr[%s] is null", dbPath, accAddr))
	}
	if info, err := os.Stat(dbPath); err != nil {
		panic(err)
	} else if !info.IsDir() {
		panic("should pass dir path")
	}

	var oncePrint bool
	// 1. open db and init key
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		fmt.Println("open db err: ", err)
		return
	}
	balanceKey := fmt.Sprintf("B/%s", accAddr)
	//balanceKey := fmt.Sprintf("%s#balance", accAddr) //address+"#balance"
	underlayDBBalanceKey := append(append([]byte(contrnm), '#'), balanceKey...)

	// 2. get the balance from db
	val, err := db.Get(underlayDBBalanceKey, nil)
	if err != nil && err != leveldb.ErrNotFound {
		fmt.Println("query acc[%s] balance from db failed, reason: %s", accAddr, err)
		return
	}
	amount := "0"
	if len(val) != 0 {
		amount = string(val)
	}
	balance, ok := big.NewInt(0).SetString(amount, 10)
	if !ok {
		panic(fmt.Sprintf("covert balance bytes[%s] to big.Int failed", balance))
	}
	if !oncePrint {
		fmt.Printf("before update balance: %s\n", balance.String())
	}

	// 3. update the balance in the db
	if step == 0 {
		if err = db.Delete(underlayDBBalanceKey, nil); err != nil {
			fmt.Println(fmt.Sprintf("db.Delete underlayDBBalanceKey: %s, err: %v", underlayDBBalanceKey, err))
			return
		}
	} else if step == 1 {
		if err = db.Put(underlayDBBalanceKey, []byte("100"), nil); err != nil {
			fmt.Println(fmt.Sprintf("db.Put underlayDBBalanceKey: %s, err: %v", underlayDBBalanceKey, err))
			return
		}
	}

	// 4. check update balance
	if step == 0 {
		if val, err = db.Get(underlayDBBalanceKey, nil); err != leveldb.ErrNotFound {
			panic(fmt.Sprintf("get balance should get not found error, but get another error: %s", err))
		}
		val = []byte("0")
	} else if step == 1 {
		if val, err = db.Get(underlayDBBalanceKey, nil); err != nil {
			panic(fmt.Sprintf("get balance failed, error: %s", err))
		}
	}

	if balance, ok = big.NewInt(0).SetString(string(val), 10); !ok {
		panic(fmt.Sprintf("covert balance bytes[%s] to big.Int failed", balance))
	}
	if !oncePrint {
		fmt.Printf("after update balance: %s\n", balance.String())
	}
	oncePrint = true

	if step == 0 {
		if balance.Int64() != 0 {
			panic(fmt.Sprintf("amount mismatch, expect: %d, actual: %d", 0, balance.Int64()))
		}
	} else if step == 1 {
		if balance.Int64() != 100 {
			panic(fmt.Sprintf("amount mismatch, expect: %d, actual: %d", 100, balance.Int64()))
		}
	}
}

//var (
//	dbPath = "/data/go/src/chainmaker.org/chainmaker-go/build/release/chainmaker-v2.1.0_alpha-wx-org1.chainmaker.org/data/wx-org1.chainmaker.org/state/chain1/store_state"
//	// dbPath = "D:\\develop\\workspace\\chainMaker\\chainmaker-go\\build\\release\\chainmaker-v2.1.0_alpha-wx-org2.chainmaker.org\\data\\wx-org2.chainmaker.org\\state\\chain1\\store_state"
//)
//
//func main1() {
//
//	db, err := leveldb.OpenFile(dbPath, nil)
//	if err != nil {
//		fmt.Println("open db err: ", err)
//		os.Exit(1)
//	}
//	val, err := db.Get([]byte("feifei_test_data_008#2d0e03297ff63ce802d2b8a71ee8efe17001f6c9da1816cf15540c982849520b#balance"), nil)
//
//	fmt.Printf("val is %s %x", val, val)
//	s := &util.Range{
//		Start: []byte("feifei"),
//		Limit: []byte("g"),
//	}
//	// s = &util.Range{
//	// 	Start: nil,
//	// 	Limit: nil,
//	// }
//	iter := db.NewIterator(s, nil)
//	for iter.Next() {
//		fmt.Printf("key: %s, val:%s \n", iter.Key(), iter.Value())
//	}
//}

//./test-bft -db_path \
///data/go/src/chainmaker.org/chainmaker-go/build/release/chainmaker-v2.1.0_alpha-wx-org1.chainmaker.org/data/wx-org1.chainmaker.org/state/chain1/store_state \
//-step 1 \
//-user_addr 2d0e03297ff63ce802d2b8a71ee8efe17001f6c9da1816cf15540c982849520b \
//-contract_name feifei_test_bad_data_008

//./test-bft -db_path /mnt/d/develop/workspace/chainMaker/chainmaker-go/build/release/chainmaker-v2.1.0_alpha-wx-org2.chainmaker.org/data/wx-org2.chainmaker.org/state/chain1/store_state \
//-step 1 \
//-user_addr 906bce0ee41d8b1d912f5d61f2abe6b12f6b5d34631233ec13b4496c81e2fb0c \
//-contract_name asset017
