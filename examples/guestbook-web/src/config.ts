/** Defaults target public-testnet-v1 Path B (July 2026 deploy). Override via PUBLIC_* env. */

export const DEFAULT_RPC_URL =
  import.meta.env.PUBLIC_RPC_URL?.trim() || "https://rpc-dew.fadosoft.com";

export const DEFAULT_GUESTBOOK =
  import.meta.env.PUBLIC_GUESTBOOK?.trim() ||
  "0x83bB4E539BE46503481E66094b01b854990BF84a";

export const DEFAULT_EXPLORER =
  import.meta.env.PUBLIC_EXPLORER_URL?.trim() ||
  "https://explorer-dew.fadosoft.com";

export const DEFAULT_CHAIN_ID = Number.parseInt(
  import.meta.env.PUBLIC_CHAIN_ID || "2205",
  10,
) || 2205;

export const GUESTBOOK_ABI = [
  "function totalEntries() view returns (uint256)",
  "function getEntry(uint256 id) view returns (address author, uint64 timestamp, string message)",
] as const;
