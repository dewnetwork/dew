import { ethereumChainParams, PUBLIC_NETWORK } from './network';

export type InjectedEthereum = {
  request: (args: {
    method: string;
    params?: unknown[];
  }) => Promise<unknown>;
  isMetaMask?: boolean;
};

declare global {
  interface Window {
    ethereum?: InjectedEthereum;
  }
}

export function hasInjectedProvider(): boolean {
  return typeof window !== 'undefined' && !!window.ethereum;
}

/**
 * Switch to Dew public-testnet-v1, or add it via EIP-3085 when missing (code 4902).
 * Mirrors examples/guestbook-web/src/wallet.ts ensureChain.
 */
export async function ensureDewChain(): Promise<void> {
  if (!window.ethereum) {
    throw new Error(
      'No wallet found. Install MetaMask (or another injected wallet).',
    );
  }

  const chainId = PUBLIC_NETWORK.chainIdHex;
  try {
    await window.ethereum.request({
      method: 'wallet_switchEthereumChain',
      params: [{ chainId }],
    });
  } catch (e: unknown) {
    const code = (e as { code?: number })?.code;
    // 4902 = unrecognized chain — add then switch is implied by the wallet
    if (code !== 4902) {
      // User rejection (4001) or other errors
      throw e instanceof Error ? e : new Error(String(e));
    }
    await window.ethereum.request({
      method: 'wallet_addEthereumChain',
      params: [ethereumChainParams()],
    });
  }
}

export function walletErrorMessage(err: unknown): string {
  if (err && typeof err === 'object') {
    const code = (err as { code?: number }).code;
    if (code === 4001) return 'Request rejected in wallet.';
    const msg = (err as { message?: string }).message;
    if (msg) return msg;
  }
  if (err instanceof Error) return err.message;
  return String(err);
}
