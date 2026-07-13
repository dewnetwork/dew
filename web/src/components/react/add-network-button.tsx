import { IconCheck, IconLoader2 } from '@tabler/icons-react';
import { useCallback, useState } from 'react';
import {
  ensureDewChain,
  hasInjectedProvider,
  walletErrorMessage,
} from '../../lib/wallet';
import { PUBLIC_NETWORK } from '../../lib/network';
import { MetaMaskIcon } from './metamask-icon';

type Status = 'idle' | 'pending' | 'ok' | 'error';

type Variant = 'primary' | 'secondary' | 'compact';

type Props = {
  variant?: Variant;
  className?: string;
  /** Show status line under the button (Network section). */
  showStatus?: boolean;
};

const variantClass: Record<Variant, string> = {
  primary: 'btn-primary px-6 py-3',
  secondary: 'btn-secondary px-6 py-3',
  compact: 'btn-primary !px-5 !py-2.5',
};

/**
 * EIP-3085 add/switch for Dew public-testnet-v1.
 * Always visible; missing wallet or user rejection surfaces as status error.
 */
export function AddNetworkButton({
  variant = 'primary',
  className = '',
  showStatus = true,
}: Props) {
  const [status, setStatus] = useState<Status>('idle');
  const [message, setMessage] = useState<string | null>(null);

  const onClick = useCallback(async () => {
    setStatus('pending');
    setMessage(null);

    if (!hasInjectedProvider()) {
      setStatus('error');
      setMessage(
        'No wallet detected. Install MetaMask, then retry — or add the network manually (chain ' +
          PUBLIC_NETWORK.chainId +
          ').',
      );
      return;
    }

    try {
      await ensureDewChain();
      setStatus('ok');
      setMessage(
        `${PUBLIC_NETWORK.chainName} ready (chain ${PUBLIC_NETWORK.chainId}).`,
      );
    } catch (err) {
      setStatus('error');
      setMessage(walletErrorMessage(err));
    }
  }, []);

  return (
    <div className={`inline-flex flex-col items-start gap-2 ${className}`}>
      <button
        type="button"
        onClick={onClick}
        disabled={status === 'pending'}
        className={`${variantClass[variant]} disabled:cursor-wait disabled:opacity-80`}
        aria-label={`Add ${PUBLIC_NETWORK.chainName} to MetaMask`}
      >
        {status === 'pending' ? (
          <IconLoader2 size={17} stroke={1.75} className="animate-spin" />
        ) : status === 'ok' ? (
          <IconCheck size={17} stroke={2} />
        ) : (
          <MetaMaskIcon size={18} className="shrink-0" />
        )}
        {status === 'pending'
          ? 'Confirm in wallet…'
          : status === 'ok'
            ? 'Network added'
            : 'Add to MetaMask'}
      </button>
      {showStatus && message && (
        <p
          className={`max-w-xs font-mono text-[11px] leading-snug ${
            status === 'error' ? 'text-[#f0a0a0]' : 'text-mist'
          }`}
          role={status === 'error' ? 'alert' : 'status'}
        >
          {message}
        </p>
      )}
    </div>
  );
}
