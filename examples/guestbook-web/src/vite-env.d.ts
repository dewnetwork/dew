/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly PUBLIC_RPC_URL?: string;
  readonly PUBLIC_GUESTBOOK?: string;
  readonly PUBLIC_EXPLORER_URL?: string;
  readonly PUBLIC_CHAIN_ID?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
