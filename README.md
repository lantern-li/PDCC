
长安链·ChainMaker, a blockchain platform for building secure, trustworthy value-exchange networks to power the new global digital economy.

ChainMaker aims to use standardized and modularized components to build a blockchain infrastructure that can be utilized to construct various blockchain systems for a wide-range of applications,  advancing blockchain developing from the “pre-industrial” era to the “industrial” era of automated assembly.

# Build (Vendor)

This repository vendors its Go dependencies under `vendor/` for reproducible/offline builds.

```bash
make chainmaker-vendor
```

The binary will be generated at `bin/chainmaker`.

# Run Chainmaker
## 1. Build executable binaries
```bash
make chainmaker-vendor
cp bin/chainmaker build/release/chainmaker-v2.3.8-wx-org.chainmaker.org/bin/
```
## 2. Modify configuration
```bash
vim build/release
/chainmaker-v2.3.8-wx-org.chainmaker.org/config/wx-org.chainmak
er.org/chainconfig/bc1.yml
```
Find the `scheduler settings` and update the parameters based on the desired execution mechanism.


# License

长安链·ChainMaker is made available under the Apache License, Version 2.0 (Apache-2.0), located in the [LICENSE](./LICENSE) file.

# Security Note

This repository contains private keys/certificates under `config/` for development/testing environments. Do **NOT** use them in production.

# Declaration
Project chainMaker-go is in early phase currently. Participation and contribution are highly encouraged. Issues and feedback are welcome to be submitted to [ISSUES](https://git.chainmaker.org.cn/chainmaker/chainmaker-go/-/issues).

chainmaker-go项目当前为开源初期阶段，我们鼓励开发者踊跃参与和贡献。您可以将相关反馈提交至 [ISSUES](https://git.chainmaker.org.cn/chainmaker/issue/-/issues) 
