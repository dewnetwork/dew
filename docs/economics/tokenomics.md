---
title: Tokenomics
description: DEW supply, rewards, fees, and staking economics.
category: economics
order: 10
status: draft
---

# Tokenomics

## Native asset

| Parameter      | Value                     |
| :------------- | :------------------------ |
| Name           | Dew Token                 |
| Ticker         | **DEW**                   |
| Decimals       | 18                        |
| Base unit      | wei (`1 DEW = 10^18 wei`) |
| Genesis supply | **1,000,000,000 DEW**     |

## Issuance / inflation (_tentative_)

- Start: **5%** annual inflation
- Decay: **10% relative reduction per year** until floor
- Floor: **1%** annual inflation (long-term security budget)

Block reward \(R\) is derived from the annual target and actual blocks produced.

### Reward split (_tentative_)

| Share | Destination                                                 |
| :---- | :---------------------------------------------------------- |
| 90%   | Proposer + voting validators (define exact split at freeze) |
| 10%   | Community treasury                                          |

```mermaid
pie showData
  title Block reward split (tentative)
  "Proposer + voting validators" : 90
  "Community treasury" : 10
```

## Fee market

EIP-1559 style — see [Gas and fees](../execution/gas-and-fees.md):

- **Base fee**: burned
- **Priority fee**: to proposer / validators

Net supply = genesis + issuance − burned base fees.

```mermaid
flowchart LR
  Genesis[Genesis 1B DEW] --> Supply[Circulating supply]
  Issuance[Block rewards] --> Supply
  Supply --> Burn[Base fee burn]
  Burn --> Supply
  Tips[Priority fees] --> Vals[Validators]
  Issuance --> Split[90% validators / 10% treasury]
```

## Staking

| Parameter                | Value         | Notes                                                  |
| :----------------------- | :------------ | :----------------------------------------------------- |
| Min validator self-stake | 100,000 DEW   | Tentative                                              |
| Unbonding                | 7 days        | Prefer seconds or fixed block delta — one unit in code |
| Commission               | Validator-set | Bounded range e.g. 0–100%                              |

## Design goals vs Ethereum

| Lever             | Effect                                     |
| :---------------- | :----------------------------------------- |
| Higher throughput | Lower fee pressure for same demand         |
| Burn base fee     | User fees do not only enrich validators    |
| Bonded BFT        | Capital at risk for safety                 |
| Later micro-fees  | Cheap native actions without full EVM cost |

## Open items before mainnet

- Exact per-block reward formula
- Treasury address / multisig policy
- Whether tips are proposer-only or shared with prevoters
- Max supply vs perpetual inflation floor narrative
