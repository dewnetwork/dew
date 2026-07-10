// Package consensus implements Dew-BFT: a Tendermint-style BFT engine for
// single-height finality (Phase A5).
//
// Local development uses in-process message passing (LocalCluster). Multi-machine
// transport arrives with Phase A6 (P2P).
//
// Round flow per height H, round R:
//
//	NewRound → Propose → Prevote → Precommit → Commit (or NewRound R+1)
//
// Quorum is strict >2/3 of total voting power. Invalid proposals (e.g. wrong
// state root) receive a nil prevote and are never committed by honest validators.
package consensus
