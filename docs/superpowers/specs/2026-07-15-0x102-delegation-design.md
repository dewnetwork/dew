# `0x102` Delegation + commission — design

**Status:** Approved design (2026-07-15) · **Track 2 / D3c residual**  
**Slot:** `0x102` (existing flagged staking entrypoint) · gate `EnableStaking` / `--staking`  
**Parents:** [validators.md](../../consensus/validators.md) · [precompiles § 0x102](../../execution/precompiles.md) · [tokenomics](../../economics/tokenomics.md) · [phases D3c](../../build/phases.md#d3c--staking-residuals-c4)

## Problem

C4 / D3c shipped **self-stake only**. Docs already target \(VP_i = S_{\text{self},i} + \sum S_{\text{delegated},i}\) and commission storage, but no methods, storage, or ActiveSet rules exist. Residual is open in debt / phases / upgrades.

## Goals (v1)

1. **Delegate / undelegate / withdrawDelegation** native DEW to a validator address, escrowed at `0x102`.  
2. **Voting power** = self-stake + total delegated (jailed or non-candidate → 0).  
3. **ActiveSet eligibility:** candidate + **not jailed** + **SelfStake ≥ min** + VP > 0; rank by full VP.  
4. **Commission** rate stored per validator (**0–10000 bps**); no reward/tip distribution in v1 (tokenomics draft).  
5. Keep actor rules consistent with S4 (payable → value-payer; zero-value mutations → **tx.origin**).

## Non-goals (v1)

- Reward / tip / issuance pro-rata split by commission.  
- Liquid staking tokens, redelegate in one step, multi-asset stake.  
- Slash burn % on delegated stake (jail-only remains; slash % still deferred).  
- Changing method bytes `0x00`–`0x09` layouts.  
- Public Path B staking enable (still operator opt-in).

## Locked decisions

| Point | Lock |
| :--- | :--- |
| Scope | **Power-only** + **commission storage** |
| Eligibility | **Min self-stake** required; pure-delegation cannot enter ActiveSet |
| Undelegation queue | **Per (delegator, validator)**; same `UnbondSeconds` as self |
| Self-delegation | **Forbidden** — use `bond` for self-stake |
| Delegate target | Any address (power idle until target is candidate + self ≥ min) |
| Commission | `uint16` bps in storage as u256; max **10000** (100%); default **0** |
| Gate | Existing `EnableStaking` only (no new flag) |
| Hardfork | **No new address**; additive methods on `0x102` when staking on |

## Method bytes (additive)

| Method | Byte | Input | Value | Gas | Actor |
| :--- | :--- | :--- | :--- | --: | :--- |
| Delegate | `0x0a` | `validator 20` | amount | 50_000 | value-payer |
| Undelegate | `0x0b` | `validator 20 \|\| amount u256` | 0 | 40_000 | tx.origin |
| WithdrawDelegation | `0x0c` | `validator 20` | 0 | 40_000 | tx.origin |
| SetCommission | `0x0d` | `bps u256` (0–10000) | 0 | 30_000 | tx.origin (= validator) |
| GetDelegation | `0x0e` | `validator 20 \|\| delegator 20` | 0 | 2_000 | — |
| GetCommission | `0x0f` | `validator 20` | 0 | 2_000 | — |
| GetDelegatedTotal | `0x10` | `validator 20` | 0 | 2_000 | — |
| PendingUndelegation | `0x11` | `validator 20 \|\| delegator 20` | 0 | 2_000 | — |

Returns: amounts as left-padded u256; pending = `amount\|\|unlockAt` (64 bytes).

## Storage (under `0x102`)

```
keccak256("dew/stake/v1/del" || validator || delegator)     → amount
keccak256("dew/stake/v1/delTot" || validator)                → total delegated
keccak256("dew/stake/v1/comm" || validator)                  → commission bps
keccak256("dew/stake/v1/delUnbondAmt" || validator || del)   → pending amount
keccak256("dew/stake/v1/delUnbondAt" || validator || del)    → unlock unix
```

Existing self / candidate / unbond self slots unchanged.

## Semantics

### Delegate

1. CALLVALUE already at `0x102`.  
2. Reject zero amount; reject `validator == value-payer` (no self-delegate).  
3. Credit `del[val][delegator] += amount`, `delTot[val] += amount`.

### Undelegate

1. Reduce live delegation; queue into per-pair pending (stack amounts; unlock = max(old, now+period)).  
2. Funds stay at module until withdraw.

### WithdrawDelegation

1. If `now ≥ unlockAt` and pending > 0, clear queue and transfer amount to **tx.origin**.

### SetCommission

1. `bps ≤ 10000`; store for `tx.origin`.  
2. No requirement to be candidate (can set before bonding).

### VotingPower / ActiveSet

```
if jailed || !candidate: VP = 0
else: VP = self + delTot

ActiveSet: candidate && !jailed && self ≥ min && VP > 0
rank by VP desc, address asc tie-break; cap K
```

### Jail

Unchanged: zero VP (via jailed bit). Delegators may still undelegate / withdraw.

## Acceptance

- [x] Module unit tests: delegate, rank ActiveSet with delegation, undelegate queue, commission bounds, no self-delegate  
- [x] Precompile tests: methods + self-delegate reject + withdraw path  
- [x] Node lab: `TestDelegationLab_Scenario`  
- [x] Docs: precompiles, validators, debt, phases D3c, tokenomics commission row  

## Out of scope follow-ups

- Reward distribution using commission  
- Redelegate without full unbond wait  
- Slash % on self and/or delegated stake  
