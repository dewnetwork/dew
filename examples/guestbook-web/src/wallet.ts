import { BrowserProvider, Contract, getAddress, isAddress } from "ethers";
import { GUESTBOOK_ABI, MAX_MESSAGE_BYTES } from "./config";

declare global {
  interface Window {
    ethereum?: {
      request: (args: { method: string; params?: unknown[] }) => Promise<unknown>;
      on?: (event: string, handler: (...args: unknown[]) => void) => void;
      removeListener?: (event: string, handler: (...args: unknown[]) => void) => void;
      isMetaMask?: boolean;
    };
  }
}

export function hasInjectedProvider(): boolean {
  return typeof window !== "undefined" && !!window.ethereum;
}

export async function connectWallet(): Promise<string> {
  if (!window.ethereum) {
    throw new Error("No wallet found. Install MetaMask (or another injected wallet).");
  }
  const accounts = (await window.ethereum.request({
    method: "eth_requestAccounts",
  })) as string[];
  if (!accounts?.length) {
    throw new Error("No account returned from wallet");
  }
  return getAddress(accounts[0]);
}

export async function ensureChain(
  chainId: number,
  rpcUrl: string,
  explorerUrl: string,
): Promise<void> {
  if (!window.ethereum) {
    throw new Error("No wallet found");
  }
  const hexId = "0x" + chainId.toString(16);
  try {
    await window.ethereum.request({
      method: "wallet_switchEthereumChain",
      params: [{ chainId: hexId }],
    });
  } catch (e: unknown) {
    const code = (e as { code?: number })?.code;
    // 4902 = chain not added
    if (code !== 4902) {
      throw e instanceof Error ? e : new Error(String(e));
    }
    await window.ethereum.request({
      method: "wallet_addEthereumChain",
      params: [
        {
          chainId: hexId,
          chainName: "Dew public-testnet-v1",
          nativeCurrency: { name: "DEW", symbol: "DEW", decimals: 18 },
          rpcUrls: [rpcUrl],
          blockExplorerUrls: explorerUrl ? [explorerUrl] : [],
        },
      ],
    });
  }
}

function assertMessage(message: string): string {
  const trimmed = message.trim();
  if (!trimmed) {
    throw new Error("Message is empty");
  }
  const byteLen = new TextEncoder().encode(trimmed).length;
  if (byteLen > MAX_MESSAGE_BYTES) {
    throw new Error(`Message too long (${byteLen} > ${MAX_MESSAGE_BYTES} bytes)`);
  }
  return trimmed;
}

export async function signGuestbook(
  guestbook: string,
  message: string,
  chainId: number,
  rpcUrl: string,
  explorerUrl: string,
): Promise<string> {
  const trimmed = assertMessage(message);
  if (!isAddress(guestbook)) {
    throw new Error("Invalid Guestbook address");
  }

  await ensureChain(chainId, rpcUrl, explorerUrl);
  if (!window.ethereum) {
    throw new Error("No wallet found");
  }
  // Network already switched; let the provider detect the active chain.
  const provider = new BrowserProvider(window.ethereum);
  const signer = await provider.getSigner();
  const book = new Contract(guestbook, GUESTBOOK_ABI, signer);
  const tx = await book.sign(trimmed);
  const receipt = await tx.wait();
  const hash = receipt?.hash ?? tx.hash;
  if (!hash) {
    throw new Error("Transaction sent but no hash returned");
  }
  return hash as string;
}

export type BurstResult = {
  hashes: [string, string];
  blockNumbers: [number | null, number | null];
  sameBlock: boolean;
};

/**
 * Send two sign() txs with consecutive nonces without waiting for the first receipt
 * before submitting the second — showcases C1 multi-tx packing when both land in one block.
 * MetaMask will prompt twice; confirm both quickly while the first is still pending.
 */
export async function burstSignGuestbook(
  guestbook: string,
  message: string,
  chainId: number,
  rpcUrl: string,
  explorerUrl: string,
): Promise<BurstResult> {
  const base = assertMessage(message);
  const msgA = `${base} · A`;
  const msgB = `${base} · B`;
  for (const m of [msgA, msgB]) {
    const n = new TextEncoder().encode(m).length;
    if (n > MAX_MESSAGE_BYTES) {
      throw new Error(`Burst message too long after suffix (${n} > ${MAX_MESSAGE_BYTES} bytes)`);
    }
  }
  if (!isAddress(guestbook)) {
    throw new Error("Invalid Guestbook address");
  }

  await ensureChain(chainId, rpcUrl, explorerUrl);
  if (!window.ethereum) {
    throw new Error("No wallet found");
  }
  const provider = new BrowserProvider(window.ethereum);
  const signer = await provider.getSigner();
  const book = new Contract(guestbook, GUESTBOOK_ABI, signer);

  // Explicit nonces so MetaMask cannot reorder / gap if the user delays the second prompt.
  const from = await signer.getAddress();
  const nonce = await provider.getTransactionCount(from, "pending");

  const tx1 = await book.sign(msgA, { nonce });
  const tx2 = await book.sign(msgB, { nonce: nonce + 1 });

  const [r1, r2] = await Promise.all([tx1.wait(), tx2.wait()]);
  const h1 = (r1?.hash ?? tx1.hash) as string;
  const h2 = (r2?.hash ?? tx2.hash) as string;
  if (!h1 || !h2) {
    throw new Error("Burst sent but missing tx hash");
  }
  const b1 = r1?.blockNumber != null ? Number(r1.blockNumber) : null;
  const b2 = r2?.blockNumber != null ? Number(r2.blockNumber) : null;
  return {
    hashes: [h1, h2],
    blockNumbers: [b1, b2],
    sameBlock: b1 != null && b2 != null && b1 === b2,
  };
}
