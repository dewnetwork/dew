# `0x102` Reward/tip split + slash burn — design

**Status:** Approved (2026-07-15) · **Track 2 / D3c residual**  
**Slot:** `0x102` · gate `EnableStaking` / `--staking`  
**Parents:** [0x102 delegation](./2026-07-15-0x102-delegation-design.md) · [tokenomics](../../economics/tokenomics.md) · [slashing](../../consensus/slashing.md)

## Problem

1. Tips (EVM priority fee) and DewTx flat fees go **100% to coinbase/proposer**. Commission bps is stored but unused.  
2. Double-sign **jails only** — no stake burn despite [slashing.md](../../consensus/slashing.md) tentative tables.  
3. Issuance / block reward still **not on-chain** (tokenomics draft).

## Goals

1. When staking is **on**, split proposer tip/fee by **self + delegated** with **commission bps**.  
2. Delegator share via **reward-per-share index** + **claimRewards** (no full delegator enumeration).  
3. Double-sign: **burn self** + **haircut delegated** + jail (provisional bps in `params`).  
4. Keep public Path B staking default **off**.

## Non-goals

- Block **issuance / inflation** mint (same split function later).  
- Downtime slash window / temporary jail (still design-only).  
- Redelegate, liquid staking, treasury 10% cut of issuance.  
- Changing tip path when staking is **off** (still 100% coinbase).

## Locked decisions

| Point | Lock |
| :--- | :--- |
| Tip/fee when staking **off** | Unchanged — 100% proposer |
| Tip/fee when staking **on** | Pro-rata \(S+D\) + commission; see formula |
| Commission | On **delegator pool only**: \(c =\) bps/10000 |
| Delegator payout | **Reward index** + claim; not per-tx enumeration |
| Claim actor | **tx.origin** (S4 fail-closed) |
| Slash trigger | Existing `0x06` jail path (dual-vote evidence) |
| Double-sign self burn | **10000 bps (100%)** provisional |
| Double-sign del burn | **500 bps (5%)** provisional via **exchange rate** |
| Already-jailed | Idempotent: no second burn |
| Numbers home | `params/staking.go` (testnet provisional; mainnet re-freeze later) |
| Issuance | **Out** — document only |

## Formula — tip/fee split

Let \(T\) = tip or DewTx fee, \(S\) = self-stake, \(D\) = **effective** delegated total, \(c\) = commission bps, \(P = S+D\).

If \(P = 0\): all \(T\) → proposer.

Else:

\[
V = T \cdot \frac{S \cdot 10000 + D \cdot c}{P \cdot 10000},\quad
R = T - V
\]

- \(V\) → proposer balance immediately.  
- \(R\) → module escrow; increases validator **reward index** by \(R \cdot 10^{18} / \text{shares}\).  
- Delegator \(i\) pending \(\approx \text{shares}_i \cdot \text{idx} / 10^{18} - \text{debt}_i\) (settle on touch / claim).

## Formula — slash (double-sign)

On verified evidence (not already jailed):

1. \(\text{selfBurn} = S \cdot 10000 / 10000\) → clear that self-stake; `SubBalance` module.  
2. \(\text{delBurn} = D \cdot 500 / 10000\); reduce **del exchange rate** so effective \(D' = D - \text{delBurn}\); burn from module.  
3. Jail + clear candidate (existing).  

**Exchange rate:** live delegation storage is **shares**; effective wei \(= \text{shares} \cdot \text{rate} / 10^{18}\). Default rate \(10^{18}\). Slash multiplies rate by \((10000 - 500)/10000\). New delegates mint shares at current rate so effective credit equals CALLVALUE. Pending undelegation amounts stay **wei** locked at undelegate time (already left live stake).

## Method bytes (additive)

| Method | Byte | Input | Gas | Actor |
| :--- | :--- | :--- | --: | :--- |
| ClaimRewards | `0x12` | `validator 20` | 40_000 | tx.origin |
| PendingRewards | `0x13` | `val 20 \|\| del 20` | 2_000 | — |

## Storage (additive under `0x102`)

```
keccak256("dew/stake/v1/delRate" || val)           → exchange rate (0 ⇒ 1e18)
keccak256("dew/stake/v1/rewIdx" || val)            → reward per share × 1e18
keccak256("dew/stake/v1/rewDebt" || val || del)    → debt
keccak256("dew/stake/v1/rewPend" || val || del)    → settled unclaimed wei
```

Existing `del` / `delTot` slots hold **shares** (1:1 with wei while rate = 1e18).

## Integration

| Path | Behavior when `EnableStaking` |
| :--- | :--- |
| EVM tip (`executor` coinbase credit) | `DistributeProposerIncome` |
| DewTx fee sink | same |
| `0x06` jail | `SlashAndJail` then jail bit |

## Acceptance

- [x] Module: tip split math (self-only, 50/50 + 10% commission, claim)  
- [x] Module: slash self 100% + del 5%; VP/effective del; second jail no double burn  
- [x] Precompile: claimRewards / pendingRewards; jail burns module balance  
- [x] DistributeProposerIncome wired for EVM tip + DewTx fee when staking on  
- [x] Docs: tokenomics, slashing, precompiles, validators, debt, phases  

## Out of scope follow-ups

- Issuance mint + treasury 10%  
- Downtime slash  
- Governance-updatable slash params without redeploy  
