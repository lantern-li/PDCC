总局注册规则：
./cmc client contract user invoke \
--contract-name=TX_ASSIGN \
--method=RegisterRule \
--sdk-conf-path=./testdata/sdk_config_admin.yml \
--org-id=wx-org1.chainmaker.org \
--params="{\"rule_type\":\"1\",\"rule\":\"{\\\"status\\\":0,\\\"contract_name\\\":\\\"RECEPIT\\\",\\\"method\\\":\\\"Save\\\",\\\"index\\\":0,\\\"offset\\\":3,\\\"name\\\":\\\"alias_name\\\"}\"}" \
--sync-result=true

./cmc client contract user get \
--contract-name=TX_ASSIGN \
--method=GetRules \
--sdk-conf-path=./testdata/sdk_config_admin.yml \
--org-id=wx-org1.chainmaker.org \
--params="{\"rule_type\":\"1\",\"contract_name\":\"RECEPIT\",\"method\":\"Save\"}"



订阅区块：
总局订阅：
./cmc block-with-rule \
--sdk-conf-path=./testdata/sdk_config_admin.yml \
--start=0 \
--end=-1 \
--with-rwset=true \
--only-header=false \
--rule-type="1" \
--contract-name=RECEPIT \
--method=Save

省订阅：
./cmc block-with-rule \
--sdk-conf-path=./testdata/sdk_config_144.yml \
--start=0 \
--end=-1 \
--with-rwset=true \
--only-header=false \
--rule-type="1" \
--contract-name=RECEPIT \
--method=Save

市订阅：
./cmc block-with-rule \
--sdk-conf-path=./testdata/sdk_config_14433.yml \
--start=0 \
--end=-1 \
--with-rwset=true \
--only-header=false \
--rule-type="1" \
--contract-name=RECEPIT \
--method=Save

区订阅：
./cmc block-with-rule \
--sdk-conf-path=./testdata/sdk_config_1443322.yml \
--start=0 \
--end=-1 \
--with-rwset=true \
--only-header=false \
--rule-type="1" \
--contract-name=RECEPIT \
--method=Save

自然人订阅：
./cmc block-with-rule \
--sdk-conf-path=./testdata/sdk_config_412233200011260810.yml \
--start=0 \
--end=-1 \
--with-rwset=true \
--only-header=false \
--rule-type="1" \
--contract-name=RECEPIT \
--method=Save


总局上链：
./cmc client contract user invoke \
--contract-name=RECEPIT \
--method=Save \
--sdk-conf-path=./testdata/sdk_config_admin.yml \
--params="{\"bizId\":\"test001\",\"businessType\":\"Test001\",\"timestamp\":\"6543234\",\"wPars\":\"14400000000\",\"rPars\":\"13100000000\",\"mutableContent\":\"412233200011260810x\",\"immutableContent\":\"412233200011260810x\",\"alias_name\":\"00000000000\"}" \
--sync-result=true

./cmc client contract user invoke \
--contract-name=RECEPIT \
--method=Save \
--sdk-conf-path=./testdata/sdk_config_admin.yml \
--params="{\"bizId\":\"test001\",\"businessType\":\"Test001\",\"timestamp\":\"6543234\",\"wPars\":\"14400000000\",\"rPars\":\"13100000000\",\"mutableContent\":\"412233200011260810x\",\"immutableContent\":\"412233200011260810x\",\"alias_name\":\"00000001234\"}" \
--sync-result=true


自然人上链：
./cmc client contract user invoke \
--contract-name=RECEPIT \
--method=Save \
--sdk-conf-path=./testdata/sdk_config_412233200011260810.yml \
--params="{\"bizId\":\"test001\",\"businessType\":\"Test001\",\"timestamp\":\"6543234\",\"wPars\":\"14400000000\",\"rPars\":\"13100000000\",\"mutableContent\":\"412233200011260810x\",\"immutableContent\":\"412233200011260810x\",\"alias_name\":\"412233200011260810x\"}" \
--sync-result=true

省级上链：
./cmc client contract user invoke \
--contract-name=RECEPIT \
--method=Save \
--sdk-conf-path=./testdata/sdk_config_144.yml \
--params="{\"bizId\":\"test001\",\"businessType\":\"Test001\",\"timestamp\":\"6543234\",\"wPars\":\"14400000000\",\"rPars\":\"13100000000\",\"mutableContent\":\"14400000000\",\"immutableContent\":\"14400000000\",\"alias_name\":\"14400000000\"}" \
--sync-result=true

市级上链：
./cmc client contract user invoke \
--contract-name=RECEPIT \
--method=Save \
--sdk-conf-path=./testdata/sdk_config_14433.yml \
--params="{\"bizId\":\"test001\",\"businessType\":\"Test001\",\"timestamp\":\"6543234\",\"wPars\":\"14400000000\",\"rPars\":\"13100000000\",\"mutableContent\":\"14433000000\",\"immutableContent\":\"14433000000\",\"alias_name\":\"14433000000\"}" \
--sync-result=true


区级上链：
./cmc client contract user invoke \
--contract-name=RECEPIT \
--method=Save \
--sdk-conf-path=./testdata/sdk_config_1443322.yml \
--params="{\"bizId\":\"test001\",\"businessType\":\"Test001\",\"timestamp\":\"6543234\",\"wPars\":\"14400000000\",\"rPars\":\"13100000000\",\"mutableContent\":\"14433220000\",\"immutableContent\":\"14433220000\",\"alias_name\":\"14433220000\"}" \
--sync-result=true

./cmc client contract user invoke \
--contract-name=RECEPIT \
--method=Save \
--sdk-conf-path=./testdata/sdk_config_1443322.yml \
--params="{\"bizId\":\"test001\",\"businessType\":\"Test001\",\"timestamp\":\"6543234\",\"wPars\":\"14400000000\",\"rPars\":\"13100000000\",\"mutableContent\":\"14433221111\",\"immutableContent\":\"14433221111\",\"alias_name\":\"14433221111\"}" \
--sync-result=true

订阅交易：

总局交易：
18309ebeb6eb9fe0ca28d1559a90e7f887f482aaa5224685838d3c516048ea38

自然人交易：
18309ac3c4652c58cab517de79b85d3741882392079c4fb18a9d53304d51e78e
18309bdb1924c060ca87fd5118928c559f1697d73dd640b6912cd40db266df34

区级交易：
18309c032d4f5de8ca51c11806f9b09a320a72ca787e419296c5dbc815faabb0
18309beac07c9e78cabf14e414cc419be2031491169542cb9977101687c12b9a

市级交易：
18309be6bdedb6f0ca15589e55f9311cc6d79ee8a16242ab825ebc6b992f2121

省级交易：
18309be238e18d50ca4a8e05198dd00d1cdc4c6ab5af49c1bd6cee7bde58cd9c

总局订阅：
./cmc sub tx-with-rule \
--sdk-conf-path=./testdata/sdk_config_admin.yml \
--start=0 \
--end=-1 \
--rule-type="1" \
--contract-name=RECEPIT \
--method=Save \
--tx-ids=\"18309ebeb6eb9fe0ca28d1559a90e7f887f482aaa5224685838d3c516048ea38,18309ac3c4652c58cab517de79b85d3741882392079c4fb18a9d53304d51e78e,18309c032d4f5de8ca51c11806f9b09a320a72ca787e419296c5dbc815faabb0,18309be6bdedb6f0ca15589e55f9311cc6d79ee8a16242ab825ebc6b992f2121,18309be238e18d50ca4a8e05198dd00d1cdc4c6ab5af49c1bd6cee7bde58cd9c\"

省订阅：
./cmc sub tx-with-rule \
--sdk-conf-path=./testdata/sdk_config_144.yml \
--start=0 \
--end=-1 \
--rule-type="1" \
--contract-name=RECEPIT \
--method=Save \
--tx-ids=\"18309ebeb6eb9fe0ca28d1559a90e7f887f482aaa5224685838d3c516048ea38,18309ac3c4652c58cab517de79b85d3741882392079c4fb18a9d53304d51e78e,18309c032d4f5de8ca51c11806f9b09a320a72ca787e419296c5dbc815faabb0,18309be6bdedb6f0ca15589e55f9311cc6d79ee8a16242ab825ebc6b992f2121,18309be238e18d50ca4a8e05198dd00d1cdc4c6ab5af49c1bd6cee7bde58cd9c\"

市订阅：
./cmc sub tx-with-rule \
--sdk-conf-path=./testdata/sdk_config_14433.yml \
--start=0 \
--end=-1 \
--rule-type="1" \
--contract-name=RECEPIT \
--method=Save \
--tx-ids=\"18309ebeb6eb9fe0ca28d1559a90e7f887f482aaa5224685838d3c516048ea38,18309ac3c4652c58cab517de79b85d3741882392079c4fb18a9d53304d51e78e,18309c032d4f5de8ca51c11806f9b09a320a72ca787e419296c5dbc815faabb0,18309be6bdedb6f0ca15589e55f9311cc6d79ee8a16242ab825ebc6b992f2121,18309be238e18d50ca4a8e05198dd00d1cdc4c6ab5af49c1bd6cee7bde58cd9c\"


区订阅：
./cmc sub tx-with-rule \
--sdk-conf-path=./testdata/sdk_config_1443322.yml \
--start=0 \
--end=-1 \
--rule-type="1" \
--contract-name=RECEPIT \
--method=Save \
--tx-ids=\"18309ebeb6eb9fe0ca28d1559a90e7f887f482aaa5224685838d3c516048ea38,18309ac3c4652c58cab517de79b85d3741882392079c4fb18a9d53304d51e78e,18309c032d4f5de8ca51c11806f9b09a320a72ca787e419296c5dbc815faabb0,18309be6bdedb6f0ca15589e55f9311cc6d79ee8a16242ab825ebc6b992f2121,18309be238e18d50ca4a8e05198dd00d1cdc4c6ab5af49c1bd6cee7bde58cd9c\"

自然人订阅：
./cmc sub tx-with-rule \
--sdk-conf-path=./testdata/sdk_config_412233200011260810.yml \
--start=0 \
--end=-1 \
--rule-type="1" \
--contract-name=RECEPIT \
--method=Save \
--tx-ids=\"18309ebeb6eb9fe0ca28d1559a90e7f887f482aaa5224685838d3c516048ea38,18309ac3c4652c58cab517de79b85d3741882392079c4fb18a9d53304d51e78e,18309c032d4f5de8ca51c11806f9b09a320a72ca787e419296c5dbc815faabb0,18309be6bdedb6f0ca15589e55f9311cc6d79ee8a16242ab825ebc6b992f2121,18309be238e18d50ca4a8e05198dd00d1cdc4c6ab5af49c1bd6cee7bde58cd9c\"



订阅事件：

测试订阅合约事件不能用RECEPIT、save交易，因为没有抛出事件

./cmc client contract user invoke \
--contract-name=TX_ASSIGN \
--method=RegisterRule \
--sdk-conf-path=./testdata/sdk_config_admin.yml \
--org-id=wx-org1.chainmaker.org \
--params="{\"rule_type\":\"1\",\"rule\":\"{\\\"status\\\":0,\\\"contract_name\\\":\\\"fact01\\\",\\\"method\\\":\\\"save\\\",\\\"index\\\":0,\\\"offset\\\":3,\\\"name\\\":\\\"file_name\\\"}\"}" \
--sync-result=true



总局订阅：
./cmc sub event-with-rule \
--sdk-conf-path=./testdata/sdk_config_admin.yml \
--start=0 \
--end=-1 \
--rule-type="1" \
--contract-name=fact01 \
--method=save

省订阅：
./cmc sub event-with-rule \
--sdk-conf-path=./testdata/sdk_config_144.yml \
--start=0 \
--end=-1 \
--rule-type="1" \
--contract-name=fact01 \
--method=save

市订阅：
./cmc sub event-with-rule \
--sdk-conf-path=./testdata/sdk_config_14433.yml \
--start=0 \
--end=-1 \
--rule-type="1" \
--contract-name=fact01 \
--method=save


区订阅：
./cmc sub event-with-rule \
--sdk-conf-path=./testdata/sdk_config_1443322.yml \
--start=0 \
--end=-1 \
--rule-type="1" \
--contract-name=fact01 \
--method=save

自然人订阅：
./cmc sub event-with-rule \
--sdk-conf-path=./testdata/sdk_config_412233200011260810.yml \
--start=0 \
--end=-1 \
--rule-type="1" \
--contract-name=fact01 \
--method=save


总局交易：
./cmc client contract user invoke \
--contract-name=fact01 \
--method=save \
--sdk-conf-path=./testdata/sdk_config_admin.yml \
--params="{\"file_name\":\"00000000000\",\"file_hash\":\"ab3456df5799b87c77e7f88\",\"time\":\"6543234\"}" \
--sync-result=true

省局交易：
./cmc client contract user invoke \
--contract-name=fact01 \
--method=save \
--sdk-conf-path=./testdata/sdk_config_144.yml \
--params="{\"file_name\":\"14400000000\",\"file_hash\":\"ab3456df5799b87c77e7f88\",\"time\":\"6543234\"}" \
--sync-result=true

市局交易：
./cmc client contract user invoke \
--contract-name=fact01 \
--method=save \
--sdk-conf-path=./testdata/sdk_config_14433.yml \
--params="{\"file_name\":\"14433000000\",\"file_hash\":\"ab3456df5799b87c77e7f88\",\"time\":\"6543234\"}" \
--sync-result=true

区局交易：
./cmc client contract user invoke \
--contract-name=fact01 \
--method=save \
--sdk-conf-path=./testdata/sdk_config_1443322.yml \
--params="{\"file_name\":\"14433220000\",\"file_hash\":\"ab3456df5799b87c77e7f88\",\"time\":\"6543234\"}" \
--sync-result=true

自然人局交易：
./cmc client contract user invoke \
--contract-name=fact01 \
--method=save \
--sdk-conf-path=./testdata/sdk_config_412233200011260810.yml \
--params="{\"file_name\":\"412233200011260810x\",\"file_hash\":\"ab3456df5799b87c77e7f88\",\"time\":\"6543234\"}" \
--sync-result=true
