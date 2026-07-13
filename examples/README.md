# Examples

Builder samples against Dew (chain ID **2205**).

| Path | Stack | Purpose |
| :--- | :--- | :--- |
| [foundry/](./foundry/) | Foundry (`forge` / `cast`) | Token, **Mock assets** (USDT/USDC/DAI/WETH/WBTC), **Guestbook**, multi-tx batch on chain **2205** |
| [hardhat/](./hardhat/) | Hardhat / ethers | Same contracts as Foundry (Token, Mock assets, Guestbook) |
| [guestbook-web/](./guestbook-web/) | Vite + React | **Read-only** Guestbook UI (public RPC default) |

| Doc | Link |
| :--- | :--- |
| Quick start | [docs/ops/quickstart.md](../docs/ops/quickstart.md) |
| Recipes | [docs/ops/recipes.md](../docs/ops/recipes.md) |

```bash
pnpm guestbook:dev    # http://localhost:4323
```
