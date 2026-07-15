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

export const MAX_MESSAGE_BYTES = 280;

/** type(uint256).max — root parentId from P3c Guestbook. */
export const PARENT_NONE =
  "115792089237316195423570985008687907853269984665640564039457584007913129639935";

export const REACTION_KINDS = [
  { kind: 0, emoji: "👍", label: "thumbs up" },
  { kind: 1, emoji: "❤️", label: "heart" },
  { kind: 2, emoji: "🔥", label: "fire" },
  { kind: 3, emoji: "🎉", label: "party" },
] as const;

/** P3c ABI — old pre-reaction contracts will fail getEntry decode. */
export const GUESTBOOK_ABI = [
  "function totalEntries() view returns (uint256)",
  "function PARENT_NONE() view returns (uint256)",
  "function getEntry(uint256 id) view returns (address author, uint64 timestamp, string message, uint256 parentId)",
  "function sign(string message) returns (uint256 id)",
  "function reply(uint256 parentId, string message) returns (uint256 id)",
  "function react(uint256 entryId, uint8 kind)",
  "function reactionCount(uint256 entryId, uint8 kind) view returns (uint256)",
  "function hasReacted(uint256 entryId, address who, uint8 kind) view returns (bool)",
] as const;
