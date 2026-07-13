# Changelog

All notable changes to this project are documented in this file.

Software releases use **semver** tags (`vX.Y.Z`) managed by
[Release Please](https://github.com/googleapis/release-please).
The protocol freeze tag **`public-testnet-v1`** is independent of package versions
(see [Public testnet freeze](./docs/ops/public-testnet.md)).

## [0.4.1](https://github.com/dewnetwork/dew/compare/v0.4.0...v0.4.1) (2026-07-13)


### Documentation

* document and implement GitHub Environment testnet variable injection for faucet captcha settings in release workflows ([23738ff](https://github.com/dewnetwork/dew/commit/23738ff4aee73e660679035e6cbdc46ba7b33078))

## [0.4.0](https://github.com/dewnetwork/dew/compare/v0.3.0...v0.4.0) (2026-07-13)


### Features

* **examples:** add mintable mock asset basket for Dew testnets ([b40601e](https://github.com/dewnetwork/dew/commit/b40601e37811e7d0a65cadb2e3b440d2ce9c0670))


### Bug Fixes

* close CodeQL alerts for block number overflow and guestbook XSS ([8b1bd16](https://github.com/dewnetwork/dew/commit/8b1bd1610af8d7ca771d5583563e6e4ca3db26df))


### Documentation

* **ops:** pin path B mock asset addresses and explorer known tokens ([0dd1912](https://github.com/dewnetwork/dew/commit/0dd1912b5ea3f6bafa3c854813c68a0d1eabe23b))

## [0.3.0](https://github.com/dewnetwork/dew/compare/v0.2.0...v0.3.0) (2026-07-13)


### Features

* implement version injection via ldflags and add version reporting for CLI and RPC ([8956f86](https://github.com/dewnetwork/dew/commit/8956f8603f96c2050a05021a14648067bb3157d4))


### Bug Fixes

* set GH_REPO in release workflows to prevent git repository errors in checkout-less jobs ([a73250c](https://github.com/dewnetwork/dew/commit/a73250c1e688cee3110425790668cd5e7852e1e7))


### Documentation

* close product-v1 live path B deploy checkbox ([1ce217b](https://github.com/dewnetwork/dew/commit/1ce217bc1005138d131880e02b80741917022165))

## [0.2.0](https://github.com/dewnetwork/dew/compare/v0.1.0...v0.2.0) (2026-07-13)


### Features

* add DNS-01 authentication support for Certbot with Cloudflare and update configuration workflow ([87b7f2e](https://github.com/dewnetwork/dew/commit/87b7f2eab7520c2bf5765472a8ee61924eebf86e))
* add ethers package to devDependencies ([696ca13](https://github.com/dewnetwork/dew/commit/696ca138b6cfd38afe01b6f6dc5cd78fd66919ea))
* add host/docker edge nginx configurations, Certbot automation scripts, and update deployment environment examples for public TLS support. ([26c6fc5](https://github.com/dewnetwork/dew/commit/26c6fc50fd1e11cf01c9428795894b80bcf76287))
* add markdown-it-task-lists support to documentation with custom styling ([02079b5](https://github.com/dewnetwork/dew/commit/02079b5d2e7025344c35872e8c6e1fd76d72cc9a))
* add public genesis configuration and build-time genesis selection to node deployment ([00c4f9f](https://github.com/dewnetwork/dew/commit/00c4f9fd1b44cdf827bc3a5e2309ebc309b00dd7))
* add read-only Guestbook web UI for public-testnet ([ee330f1](https://github.com/dewnetwork/dew/commit/ee330f1b9adecd03cdb660e17beb883eecc8b837))
* add runbook and infrastructure for single-host controlled public RPC deployment ([8e7801f](https://github.com/dewnetwork/dew/commit/8e7801f141c7679c0ea500d87bd68df9806761a2))
* add site build script to merge landing and documentation into a single static directory ([c4d9ffd](https://github.com/dewnetwork/dew/commit/c4d9ffd4fe508ac34658a330a204f6611adfcc47))
* automate Let's Encrypt certificate issuance and renewal via dedicated certbot entrypoint ([c5f8fdf](https://github.com/dewnetwork/dew/commit/c5f8fdfa10175e48994cbd7b5d4e45b4cb259b74))
* **b4:** add load tests/benchmarks and fix PE Finalise equivalence ([1e0111e](https://github.com/dewnetwork/dew/commit/1e0111e9586d9750df7e956b6c118a95b3e7b522))
* **b4:** freeze fee policy helpers and document tuning rationale ([caa1521](https://github.com/dewnetwork/dew/commit/caa1521047df78763cdf71cf51768a108f0dec83))
* **b4:** Phase B security audit checklist, adversarial tests, debt log ([b78f679](https://github.com/dewnetwork/dew/commit/b78f67926917b8272419d891f3ee351a7cc78e1d))
* **c1:** unified mempool admission for EVM and DewTx ([89bc8c5](https://github.com/dewnetwork/dew/commit/89bc8c5f5a1ad8ff8a24e1bf15e56ebb3d0c6088))
* **c2:** encrypt P2P sessions with X25519 and AES-GCM ([fabc17c](https://github.com/dewnetwork/dew/commit/fabc17ccc53953716015fc49c7ae0b0a704e3d32))
* **c3:** replace provisional state root with sparse Merkle tree ([ba7b222](https://github.com/dewnetwork/dew/commit/ba7b22298b86a0ebdc66162e3253d685ba604c21))
* **c4:** implement staking module and 0x102 precompile ([e5e0ed6](https://github.com/dewnetwork/dew/commit/e5e0ed68f3abc52a8736ee5c71a0a9f7e41c751b))
* **c5:** private testnet ops docs, chaos restart, encrypt default ([ef79931](https://github.com/dewnetwork/dew/commit/ef799311be14c40c6af17fe5e588181d12733f81))
* **c6:** public-testnet-v1 freeze, RPC abuse limits, and runbook ([eacfdb3](https://github.com/dewnetwork/dew/commit/eacfdb3aedc04921f2ed928b402bdbeaf4eb86de))
* **consensus:** mempool block builder and genesis valset ([6518d63](https://github.com/dewnetwork/dew/commit/6518d6377caa3608f97d4d72fe24499ee744ad8a))
* **consensus:** round runner with commit hook and timeout ([82ab8d7](https://github.com/dewnetwork/dew/commit/82ab8d7d04f0deffbd43c565028d5ef079376770))
* **d3a:** compose multi BFT layout and close D3a acceptance docs ([84c6980](https://github.com/dewnetwork/dew/commit/84c69808d8a0fe3a34491802dfb480af0f62fac0))
* **d3b:** durable peer store and auto-redial ([5e87435](https://github.com/dewnetwork/dew/commit/5e8743594f146fe72bcb499befd1d3c503528b41))
* **d3c:** double-sign evidence, nested bond, ActiveSet BFT rotation ([168befb](https://github.com/dewnetwork/dew/commit/168befbb72f373fac1a1dc4676e3d566569dd233))
* decouple public RPC deployment from private soak stack with nginx proxy and environment configurations ([1ad1c1f](https://github.com/dewnetwork/dew/commit/1ad1c1fb0f8f9ab4a44fa9e1acf9de0cf5d9bab1))
* **dew:** --bft.min-block-interval for validator pace ([c39a66b](https://github.com/dewnetwork/dew/commit/c39a66b6fd1cbbd13b11f2717825d1002321949e))
* **dew:** validator and full-node BFT stack with multi-process integration ([348071e](https://github.com/dewnetwork/dew/commit/348071eb61f866b5ba234dd2b5d591888c72e906))
* enhance documentation with math support and modernized mermaid diagram styling ([286f75b](https://github.com/dewnetwork/dew/commit/286f75b65af2ac65c2a58804b98c502cb0ae4d9a))
* **examples:** Hardhat DX kit for Dew chain 2205 ([cd14e1c](https://github.com/dewnetwork/dew/commit/cd14e1c0783728604691aa2ad994cddf5f78fc7f))
* **faucet:** expose funder balanceWei on /info for UI (P2d) ([22f2dae](https://github.com/dewnetwork/dew/commit/22f2daebab069c2e1f44f2e257475afabbe3972b))
* Guestbook demo recipes and left-pad eth_getStorageAt slots ([4157587](https://github.com/dewnetwork/dew/commit/4157587cf35f5b01f338719cb3ae33165f426192))
* **guestbook-web:** Burst ×2 multi-tx UI for C1 packing demo ([daeeb5f](https://github.com/dewnetwork/dew/commit/daeeb5f650938cf5db7b01375e96f39932a0148c))
* implement CI workflows for Go and Web stacks with updated project documentation ([9326161](https://github.com/dewnetwork/dew/commit/9326161a33e9a6c9db3b36990edcfaa7da65dec9))
* implement dynamic network gas price resolution using median effective transaction fees and update docs ([20fd249](https://github.com/dewnetwork/dew/commit/20fd249b0c434600b23a6c281ff0eba9e10abee4))
* implement encrypted wallet management and CLI for address creation and listing ([5e037ac](https://github.com/dewnetwork/dew/commit/5e037ac6c84fe25b118efbdcb97a92b30c41367a))
* implement faucet-web frontend, docker deployment configurations, and project structure updates ([1ec01f2](https://github.com/dewnetwork/dew/commit/1ec01f279aa12318619d47d62318bd5c751b05c6))
* implement multi-arch GHCR image publishing in release workflow and update project documentation ([1b67851](https://github.com/dewnetwork/dew/commit/1b6785152615b5d6168272021b0931695867178d))
* implement Phase A2 types, flat state, and genesis alloc ([55a0d92](https://github.com/dewnetwork/dew/commit/55a0d92cdffce88700404d222a0263367f2646b8))
* implement Phase A3 EVM bridge and sequential executor ([53c4c86](https://github.com/dewnetwork/dew/commit/53c4c86bce7269bfa16a03b5f9e6dc2d3ed5844a))
* implement Phase A4 JSON-RPC HTTP node for MetaMask and Foundry ([49afad3](https://github.com/dewnetwork/dew/commit/49afad30a96a932b6ee3ed3324f54aa0015d8826))
* implement Phase A5 Dew-BFT local consensus engine ([1a9e0f3](https://github.com/dewnetwork/dew/commit/1a9e0f374f08bfd7c4661a07dc836d14c62f3972))
* implement Phase A6 P2P networking ([53b3a12](https://github.com/dewnetwork/dew/commit/53b3a12288271cdc6d36864e2bf91b092f565ad3))
* implement Phase A7 local multi-validator devnet ([a1d9291](https://github.com/dewnetwork/dew/commit/a1d9291107a3f73177672d60b30ccb64ecc0b6c6))
* implement production faucet service with rate limiting and captcha support ([264e2d3](https://github.com/dewnetwork/dew/commit/264e2d35402b2fb3822002f43f0c4a92572a4044))
* implement web project scaffolding with Astro and React components ([428a94a](https://github.com/dewnetwork/dew/commit/428a94add429a27af9cab2759c9a8d74707aac3f))
* initialize Dew Explorer SPA with RPC integration, block/tx routing, and search functionality ([5245b30](https://github.com/dewnetwork/dew/commit/5245b30678945401900cc359d6f6667dbf5f4c74))
* initialize VitePress documentation site with custom theme and pnpm configuration ([682722f](https://github.com/dewnetwork/dew/commit/682722fb16e4a3baf0a9de43b4d9c90e3e3f35ae))
* integrate P2P networking in dew node and add containerized deployment support with Docker and systemd. ([ce8308c](https://github.com/dewnetwork/dew/commit/ce8308c5d22882adc37a650f4f2ad3060a7b4240))
* MetaMask sign for Guestbook and public host packaging ([56fe057](https://github.com/dewnetwork/dew/commit/56fe0574ac2f0d11620bb527a6531bfcc2a8de94))
* **native:** DewTx executor with fail-closed access lists ([96f27d6](https://github.com/dewnetwork/dew/commit/96f27d677b2c685af3de3f8b8f149428136b723b))
* **node:** add ImportCommittedBlock apply path ([a9efb2f](https://github.com/dewnetwork/dew/commit/a9efb2fef4242bb59e7e586f5f4d494ab412e0e9))
* **node:** add SetAutoMine admit-only tx path ([9ccc496](https://github.com/dewnetwork/dew/commit/9ccc496920dde6dcd77e211ec0f08354e1973b0e))
* **node:** mempool block builder and execution validator ([e4970fa](https://github.com/dewnetwork/dew/commit/e4970fab55794464a287f490c0df4055a73a8577))
* **node:** multi-tx + nonce-gap for DewTx auto-mine (C1 residual) ([a7e236e](https://github.com/dewnetwork/dew/commit/a7e236e9f5942315d01aee851c754ced5efe2d93))
* **node:** multi-tx pack and nonce-gap mempool for C1 ([1eb511a](https://github.com/dewnetwork/dew/commit/1eb511a2d8d2268499a63ae8cff27a7069c67a25))
* **node:** partial re-select when proposal sim fails mid-block ([e301d8c](https://github.com/dewnetwork/dew/commit/e301d8ccbf0f8b9f44658e7d91c5593e2cf3d1a1))
* **p2p:** BFT wire bridge and node P2PBackend ([ac79b16](https://github.com/dewnetwork/dew/commit/ac79b1687bee0449113350f5a7062fd240218964))
* **product:** product-v1 explorer ERC-20, faucet UX, Guestbook filter ([85bd58f](https://github.com/dewnetwork/dew/commit/85bd58f38e2d8eb599ef59f12e7a94087a8d0239))
* **product:** v1.1 known-token balances, wallet paste, share filter ([6816ae3](https://github.com/dewnetwork/dew/commit/6816ae329904ed0a86af6be5f028d0158a1ca2d9))
* **rpc:** add dew_sendRawTransaction and dew_getExecutionStats ([10f1f5a](https://github.com/dewnetwork/dew/commit/10f1f5a56789106bf6b99f2af07e041361585d2b))
* **staking:** enforce unbonding period with withdraw (D3c) ([34388aa](https://github.com/dewnetwork/dew/commit/34388aa89d38f6821aa15f7f3d68647692d19b62))
* **state:** add StateDB Copy, overlay merge, and access tracking for Dew-PE ([bc4eaa5](https://github.com/dewnetwork/dew/commit/bc4eaa5c4b78ce369c525c252b8ab2e98f75d8c2))
* **types:** add block wire encoding for P2P sync ([f56a9eb](https://github.com/dewnetwork/dew/commit/f56a9eb69dd838d5abc7f88ea66c6856a5b5ebe6))
* **types:** freeze DewTx codec and domain-separated signature ([3fe156c](https://github.com/dewnetwork/dew/commit/3fe156c609ac69ed00c25d7071989f7897fd3f7c))
* **vm:** add feature-flagged native transfer precompile at 0x100 ([9542c6f](https://github.com/dewnetwork/dew/commit/9542c6fc4192fdfbc6ade1389190d45c5c36b7f2))
* **vm:** implement Dew-PE optimistic parallel executor with metrics ([20ec446](https://github.com/dewnetwork/dew/commit/20ec44664cb3e9da54d1ba5a1cdb029ea5808923))
* **web:** sync landing to public-testnet-v1 and kebab-case components ([27246a0](https://github.com/dewnetwork/dew/commit/27246a0ee9b7a5108f6f567cb0eb39c91cd34fca))


### Bug Fixes

* **consensus:** default MinBlockInterval 1s for multiproc pace ([e8a7a21](https://github.com/dewnetwork/dew/commit/e8a7a2198365a129839a6fbb0407de846215c5ec))
* **d3a:** multiproc BFT catch-up, P2P write queue, ERC-20 heavy test ([8377458](https://github.com/dewnetwork/dew/commit/8377458e2c09224c5cf8d32978eb17a2cc83d5c8))
* **devnet:** harden multiproc catch-up teardown and LongEmpty diagnostics ([4df9f4a](https://github.com/dewnetwork/dew/commit/4df9f4a4e11b44e009b559cd68b6f54819de3634))
* **p2p:** drop bulk outbound when queue full; keep consensus blocking ([a335ac7](https://github.com/dewnetwork/dew/commit/a335ac7b6e8c841bee6cb61b127a13f19ee29bb4))


### Documentation

* add block explorer design specification and update testnet launch documentation ([25875fc](https://github.com/dewnetwork/dew/commit/25875fcc64addd3d6a593dce7b172921f838fefb))
* add D3 scale design spec and update development documentation and roadmap. ([fe1f95c](https://github.com/dewnetwork/dew/commit/fe1f95c76243ca91dcb989d4def9a16f14a7d820))
* add D3a multi-process BFT design spec (superpowers) ([58096c8](https://github.com/dewnetwork/dew/commit/58096c89f240ecc02e33f0f60bf2ceeff4901926))
* add D3a multi-process BFT implementation plan ([a4ce23d](https://github.com/dewnetwork/dew/commit/a4ce23d38b316405ef3a28dc2903cd7f7339c50d))
* add D3a residual private multi-staging design ([cf8319b](https://github.com/dewnetwork/dew/commit/cf8319bbd5e85e2a1818105a84b029e2ac65f44f))
* add D3a residual private multi-staging implementation plan ([7342846](https://github.com/dewnetwork/dew/commit/7342846e97515de58fc8dc944270b349f413072f))
* add Foundry DX kit and 5-minute builder quickstart ([9e41075](https://github.com/dewnetwork/dew/commit/9e410751efbdf38d22ff6ae6dfcdbf0ddfe3ccf0))
* add project development guidelines and technical debt tracking documentation ([dd4246f](https://github.com/dewnetwork/dew/commit/dd4246f5def784227acf3c844ed99fe5410bb635))
* add requirement to synchronize documentation with codebase changes in AGENTS.md ([3968fc9](https://github.com/dewnetwork/dew/commit/3968fc989b6706be228a783a2f2ec5a774cd0eae))
* close D3a residual and fix multi-process private runbook ([2363fb2](https://github.com/dewnetwork/dew/commit/2363fb2df7c6dc8bf0d1e55a76d04354f3816e8c))
* define Phase C testnet readiness roadmap ([1506487](https://github.com/dewnetwork/dew/commit/150648753db6f2e04a4c24aed9f885fec34e6c4e))
* initialize technical documentation and project architecture structure ([a1ec672](https://github.com/dewnetwork/dew/commit/a1ec672d4b8a7419167377fe0673aed16ffa0df6))
* mark D3a multi-process BFT complete in phases ([0550980](https://github.com/dewnetwork/dew/commit/05509803725f9cdfe19518797d96430484804612))
* mark Phase B complete and document DewTx, PE, precompiles ([346d40c](https://github.com/dewnetwork/dew/commit/346d40ca8df0c14f39131c5c8bff9c65283d94bd))
* promote protocol and execution pages to stable ([cd06bd2](https://github.com/dewnetwork/dew/commit/cd06bd21fd5617e8bcf21f0381e4df5013663472))
* promote remaining draft categories to stable ([31f5f1c](https://github.com/dewnetwork/dew/commit/31f5f1c795621419737a512ee9ddcdac1bcc7e8d))
* public DX surface — try-public, Guestbook product, freeze publish ([c324adb](https://github.com/dewnetwork/dew/commit/c324adb06554cef33c693b0aad13cd4396166bbc))
* update public-testnet-v1 documentation to reflect live status and path B deployment endpoints. ([96528c5](https://github.com/dewnetwork/dew/commit/96528c5eddc4ca70bfca691c0f3f05ecae1bd680))


### Code Refactoring

* implement reusable syntax-highlighted shell component for builder terminal snippets ([d56c84f](https://github.com/dewnetwork/dew/commit/d56c84f19428d1114b5eb78b5889e3f6a2552a59))
* migrate documentation mermaid rendering to vitepress-mermaid-renderer and upgrade to vitepress 2 alpha ([ab1541c](https://github.com/dewnetwork/dew/commit/ab1541c7c1b04d78ee233c3dd4369d64db537212))
* restructure project documentation into categories and update internal build/product guides ([bd7b294](https://github.com/dewnetwork/dew/commit/bd7b294473924c138a7ae14754d23263b7d50a9c))
* simplify TLS management by removing certs directory in favor of environment-based Cloudflare token configuration ([4fb81f3](https://github.com/dewnetwork/dew/commit/4fb81f31188537496e3b0d752f2365a39ab8cb1e))
* standardize test database initialization and implement durable chaindata persistence layer ([6a85e7d](https://github.com/dewnetwork/dew/commit/6a85e7d533d80b1114c257908432c4fb55af8fa5))
* update deployment documentation and compose files to default to GHCR image pulls instead of source builds ([18c69b8](https://github.com/dewnetwork/dew/commit/18c69b842f74e16d1a160c9af9437a253c635d88))

## [0.1.0](https://github.com/dewnetwork/dew/releases/tag/v0.1.0)

### Features

* Initial public-testnet-v1 monorepo baseline (Go L1, explorer, faucet, docs).
