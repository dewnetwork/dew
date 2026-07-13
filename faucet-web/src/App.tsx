import React, { useState, useEffect, useRef } from "react";

interface FaucetInfo {
  chainId: number;
  mode: "allowlist" | "captcha" | "dev";
  amountWei: string;
  from: string;
  perAddress: number;
  perAddressWindowSec: number;
  perIP: number;
  perIPWindowSec: number;
  freezeTag: string;
}

interface DripResponse {
  txHash: string;
  from: string;
  to: string;
  amount: string;
}

export default function App() {
  const [address, setAddress] = useState("");
  const [captchaToken, setCaptchaToken] = useState("");
  const [loading, setLoading] = useState(false);
  const [info, setInfo] = useState<FaucetInfo | null>(null);
  const [infoLoading, setInfoLoading] = useState(true);
  const [infoError, setInfoError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<DripResponse | null>(null);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const captchaWidgetRef = useRef<HTMLDivElement>(null);
  const widgetIdRef = useRef<string | null>(null);

  const getApiUrl = (path: string) => {
    const base = import.meta.env.PUBLIC_FAUCET_API_URL || "/api";
    return `${base.replace(/\/$/, "")}/${path.replace(/^\//, "")}`;
  };

  const getExplorerTxLink = (txHash: string) => {
    const base = import.meta.env.PUBLIC_EXPLORER_URL;
    if (!base) return null;
    return `${base.replace(/\/$/, "")}/tx/${txHash}`;
  };

  const getExplorerAddressLink = (addr: string) => {
    const base = import.meta.env.PUBLIC_EXPLORER_URL;
    if (!base) return null;
    return `${base.replace(/\/$/, "")}/address/${addr.toLowerCase()}`;
  };

  /** Map common faucet backend errors to clearer UX copy. */
  const friendlyFaucetError = (raw: string): string => {
    const m = raw.toLowerCase();
    if (m.includes("rate") || m.includes("limit") || m.includes("cooldown") || m.includes("too many")) {
      const windowHint =
        info != null
          ? ` Per address: ${info.perAddress} / ${formatSeconds(info.perAddressWindowSec)}; per IP: ${info.perIP} / ${formatSeconds(info.perIPWindowSec)}.`
          : "";
      return `Rate limit reached — try again after the cooldown window.${windowHint}`;
    }
    if (m.includes("allowlist") || m.includes("not allowed") || m.includes("not on")) {
      return "This address is not on the faucet allowlist. Ask an operator to add it, or use captcha mode if enabled.";
    }
    if (m.includes("captcha") || m.includes("turnstile") || m.includes("hcaptcha")) {
      return "Captcha verification failed. Refresh the challenge and try again.";
    }
    return raw;
  };

  const formatDewAmount = (weiStr: string) => {
    try {
      const val = BigInt(weiStr);
      const dec = 18n;
      const divisor = 10n ** dec;
      const integerPart = val / divisor;
      const fractionalPart = val % divisor;
      if (fractionalPart === 0n) {
        return `${integerPart} DEW`;
      }
      let fracStr = fractionalPart.toString().padStart(18, "0");
      fracStr = fracStr.replace(/0+$/, "");
      return `${integerPart}.${fracStr} DEW`;
    } catch {
      return "1 DEW";
    }
  };

  const formatSeconds = (sec: number) => {
    if (sec >= 86400) return `${sec / 86400} day${sec / 86400 > 1 ? "s" : ""}`;
    if (sec >= 3600) return `${sec / 3600} hour${sec / 3600 > 1 ? "s" : ""}`;
    if (sec >= 60) return `${sec / 60} minute${sec / 60 > 1 ? "s" : ""}`;
    return `${sec} seconds`;
  };

  useEffect(() => {
    fetch(getApiUrl("info"))
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      })
      .then((data) => {
        setInfo(data);
        setInfoLoading(false);
      })
      .catch((err) => {
        console.error("Failed to load faucet info:", err);
        setInfoError("Could not connect to the faucet server. Make sure the backend is running.");
        setInfoLoading(false);
      });
  }, []);

  useEffect(() => {
    if (!info || info.mode !== "captcha") return;

    const provider = import.meta.env.PUBLIC_CAPTCHA_PROVIDER || "turnstile";
    const siteKey = import.meta.env.PUBLIC_CAPTCHA_SITE_KEY;

    if (!siteKey) {
      console.warn("CAPTCHA mode enabled but site key (PUBLIC_CAPTCHA_SITE_KEY) is missing.");
      setError("Captcha configuration error: PUBLIC_CAPTCHA_SITE_KEY missing.");
      return;
    }

    const scriptId = `captcha-${provider}-script`;
    if (document.getElementById(scriptId)) {
      renderCaptcha(provider, siteKey);
      return;
    }

    const script = document.createElement("script");
    script.id = scriptId;
    script.async = true;
    script.defer = true;

    if (provider === "turnstile") {
      script.src = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";
      script.onload = () => renderCaptcha(provider, siteKey);
    } else if (provider === "hcaptcha") {
      script.src = "https://js.hcaptcha.com/1/api.js?render=explicit";
      script.onload = () => renderCaptcha(provider, siteKey);
    }

    document.head.appendChild(script);
  }, [info]);

  const renderCaptcha = (provider: string, siteKey: string) => {
    if (!captchaWidgetRef.current) return;
    resetCaptcha(provider);

    try {
      if (provider === "turnstile" && window.turnstile) {
        const id = window.turnstile.render(captchaWidgetRef.current, {
          sitekey: siteKey,
          callback: (token) => {
            setCaptchaToken(token);
            setError(null);
          },
          "expired-callback": () => setCaptchaToken(""),
          "error-callback": () => {
            setCaptchaToken("");
            setError("Captcha verification failed. Please refresh and try again.");
          },
          theme: "dark",
        });
        widgetIdRef.current = id;
      } else if (provider === "hcaptcha" && window.hcaptcha) {
        const id = window.hcaptcha.render(captchaWidgetRef.current, {
          sitekey: siteKey,
          callback: (token) => {
            setCaptchaToken(token);
            setError(null);
          },
          "expired-callback": () => setCaptchaToken(""),
          "error-callback": () => {
            setCaptchaToken("");
            setError("Captcha verification failed. Please refresh and try again.");
          },
          theme: "dark",
        });
        widgetIdRef.current = id;
      }
    } catch (e) {
      console.error("Captcha render error:", e);
    }
  };

  const resetCaptcha = (provider: string) => {
    setCaptchaToken("");
    if (widgetIdRef.current === null) return;
    try {
      if (provider === "turnstile" && window.turnstile) {
        window.turnstile.reset(widgetIdRef.current);
      } else if (provider === "hcaptcha" && window.hcaptcha) {
        window.hcaptcha.reset(widgetIdRef.current);
      }
    } catch (e) {
      console.warn("Captcha reset failed:", e);
    }
  };

  const handleAddressChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value.trim();
    setAddress(value);

    if (value === "") {
      setValidationError(null);
      return;
    }

    if (!/^0x[a-fA-F0-9]{40}$/.test(value)) {
      setValidationError("Invalid address (must start with 0x and be 40 hex characters)");
    } else {
      setValidationError(null);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (loading) return;

    setError(null);
    setSuccess(null);

    const cleanAddress = address.trim();
    if (!cleanAddress) {
      setError("Please enter a wallet address.");
      return;
    }

    if (!/^0x[a-fA-F0-9]{40}$/.test(cleanAddress)) {
      setError("Invalid Ethereum address format.");
      return;
    }

    if (info?.mode === "captcha" && !captchaToken) {
      setError("Please complete the CAPTCHA challenge.");
      return;
    }

    setLoading(true);

    try {
      const res = await fetch(getApiUrl("drip"), {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          address: cleanAddress,
          captchaToken: captchaToken,
        }),
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || "Failed to transfer tokens.");
      }

      setSuccess(data);
      setAddress("");

      if (info?.mode === "captcha") {
        const provider = import.meta.env.PUBLIC_CAPTCHA_PROVIDER || "turnstile";
        resetCaptcha(provider);
      }
    } catch (err: unknown) {
      console.error(err);
      const message = err instanceof Error ? err.message : "An unexpected error occurred.";
      setError(friendlyFaucetError(message));
    } finally {
      setLoading(false);
    }
  };

  const [copiedField, setCopiedField] = useState<"from" | "tx" | null>(null);

  const copyToClipboard = (text: string, field: "from" | "tx" = "from") => {
    void navigator.clipboard.writeText(text);
    setCopiedField(field);
    setCopied(true);
    setTimeout(() => {
      setCopied(false);
      setCopiedField(null);
    }, 2000);
  };

  const year = new Date().getFullYear();
  const freezeLabel = infoLoading ? "Connecting…" : info?.freezeTag || "public-testnet-v1";

  return (
    <div className="relative flex min-h-dvh flex-col overflow-x-hidden">
      <div className="grain" aria-hidden="true" />

      {/* Ambient dew beads — matches landing hero accent */}
      <div className="pointer-events-none absolute inset-0 opacity-50" aria-hidden="true">
        <div className="dew-bead absolute top-[18%] left-[8%] h-1.5 w-1.5 rounded-full bg-cyan/50 blur-[0.5px]" />
        <div className="dew-bead dew-bead-delay-1 absolute top-[32%] left-[22%] h-1 w-1 rounded-full bg-mist/40" />
        <div className="dew-bead dew-bead-delay-2 absolute top-[14%] right-[28%] h-2 w-2 rounded-full bg-cyan/30 blur-[1px]" />
      </div>

      <header className="site-nav relative z-20">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-4 px-5 sm:px-8">
          <a href="/" className="group flex items-center gap-2.5" aria-label="Dew Faucet home">
            <img
              src="/logo.svg"
              alt=""
              width={28}
              height={28}
              className="rounded-lg ring-1 ring-[var(--color-line)] transition group-hover:ring-cyan/40"
            />
            <span className="font-display text-[1.05rem] font-bold tracking-tight text-frost">
              Dew<span className="text-cyan">Faucet</span>
            </span>
          </a>
          <span className="eyebrow !py-1 !text-[10px]">
            <span className="relative flex h-1.5 w-1.5">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-success opacity-60" />
              <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-success" />
            </span>
            {freezeLabel}
          </span>
        </div>
      </header>

      <main className="relative z-10 flex flex-1 flex-col items-center px-5 py-12 sm:px-8 sm:py-16">
        <div className="w-full max-w-xl">
          <div className="mb-8 text-center">
            <p className="eyebrow mb-5">
              <svg
                className="h-3.5 w-3.5 text-cyan"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.75"
                aria-hidden="true"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M12 3.25c.4 2.2 1.7 3.8 3.5 5.1 1.6 1.1 2.75 2.5 2.75 4.4A6.25 6.25 0 0112 19.15 6.25 6.25 0 015.75 12.75c0-1.9 1.15-3.3 2.75-4.4 1.8-1.3 3.1-2.9 3.5-5.1z"
                />
              </svg>
              Testnet drip
            </p>
            <h1 className="font-display text-[2.15rem] leading-[1.1] font-extrabold tracking-[-0.03em] text-frost text-balance sm:text-4xl">
              Claim test{" "}
              <span className="bg-gradient-to-r from-frost via-cyan to-mist bg-clip-text text-transparent">
                DEW
              </span>
            </h1>
            <div className="mx-auto mt-5 h-0.5 w-20 rounded-full dew-edge" aria-hidden="true" />
            <p className="mx-auto mt-5 max-w-md text-base leading-relaxed text-slate">
              Request a small amount of test DEW to deploy contracts and send transactions on the
              Dew public testnet.
            </p>
          </div>

          <div className="card-surface p-6 sm:p-8">
            {infoError && (
              <div className="alert alert-error mb-6 text-center" role="alert">
                {infoError}
              </div>
            )}

            {error && (
              <div className="alert alert-error mb-6 flex items-start gap-3" role="alert">
                <svg
                  className="mt-0.5 h-5 w-5 shrink-0 text-danger"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  aria-hidden="true"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
                <span>{error}</span>
              </div>
            )}

            {success && (
              <div className="alert alert-success mb-6 space-y-4" role="status">
                <div className="flex items-center gap-3">
                  <div className="flex h-8 w-8 items-center justify-center rounded-full border border-[rgb(52_211_153_/_0.3)] bg-[rgb(52_211_153_/_0.15)] text-success">
                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                    </svg>
                  </div>
                  <div>
                    <h3 className="font-semibold text-frost">Tokens sent</h3>
                    <p className="text-xs text-muted">
                      Successfully dripped {formatDewAmount(success.amount)}
                    </p>
                  </div>
                </div>

                <div className="space-y-1.5 rounded-lg border border-[var(--color-line)] bg-ink/50 p-3 font-mono text-xs">
                  <div className="flex items-center justify-between gap-3">
                    <span className="text-muted">Recipient</span>
                    <span className="flex min-w-0 items-center gap-2">
                      <span className="truncate text-slate max-w-[200px]">{success.to}</span>
                      {getExplorerAddressLink(success.to) ? (
                        <a
                          href={getExplorerAddressLink(success.to)!}
                          target="_blank"
                          rel="noreferrer"
                          className="shrink-0 text-cyan hover:text-mist"
                        >
                          view
                        </a>
                      ) : null}
                    </span>
                  </div>
                  <div className="flex items-center justify-between gap-3">
                    <span className="text-muted">Tx hash</span>
                    <span className="flex min-w-0 items-center gap-2">
                      <span className="truncate font-medium text-cyan max-w-[160px]">
                        {success.txHash}
                      </span>
                      <button
                        type="button"
                        onClick={() => copyToClipboard(success.txHash, "tx")}
                        className="btn-ghost shrink-0 !px-2 !py-0.5 text-[10px]"
                      >
                        {copied && copiedField === "tx" ? "Copied" : "Copy"}
                      </button>
                    </span>
                  </div>
                </div>

                <div className="flex flex-wrap items-center gap-3">
                  {getExplorerTxLink(success.txHash) ? (
                    <a
                      href={getExplorerTxLink(success.txHash)!}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-2 text-xs font-semibold text-cyan hover:text-mist"
                    >
                      View tx on explorer
                      <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                        />
                      </svg>
                    </a>
                  ) : (
                    <p className="text-[10px] text-muted italic">No explorer link configured.</p>
                  )}
                  {getExplorerAddressLink(success.to) ? (
                    <a
                      href={getExplorerAddressLink(success.to)!}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-2 text-xs font-medium text-slate hover:text-cyan"
                    >
                      Recipient address
                    </a>
                  ) : null}
                </div>
              </div>
            )}

            <form onSubmit={handleSubmit} className="space-y-6">
              <div>
                <label htmlFor="address" className="mb-2 block text-sm font-medium text-frost">
                  Wallet address
                </label>
                <input
                  type="text"
                  id="address"
                  disabled={loading}
                  value={address}
                  onChange={handleAddressChange}
                  placeholder="0x…"
                  autoComplete="off"
                  spellCheck={false}
                  className={`field-input ${validationError ? "is-invalid" : ""}`}
                />
                {validationError && (
                  <p className="mt-1.5 text-xs font-medium text-danger">{validationError}</p>
                )}
              </div>

              {info?.mode === "captcha" && (
                <div className="flex flex-col items-center justify-center rounded-xl border border-[var(--color-line)] bg-ink/40 p-4">
                  <div id="captcha-widget" ref={captchaWidgetRef} className="captcha-container" />
                  <p className="mt-2 text-center text-[10px] text-muted">
                    Cloudflare Turnstile or hCaptcha verification required to limit automated requests.
                  </p>
                </div>
              )}

              {info?.mode === "allowlist" && (
                <div className="alert alert-info flex items-start gap-2.5 text-xs">
                  <svg
                    className="mt-0.5 h-4 w-4 shrink-0 text-cyan"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    aria-hidden="true"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                  <span>
                    <strong className="text-frost">Allowlist mode:</strong> only pre-approved
                    testnet addresses can receive tokens.
                  </span>
                </div>
              )}

              <button
                type="submit"
                disabled={
                  loading || !!validationError || (info?.mode === "captcha" && !captchaToken)
                }
                className="btn-primary"
              >
                {loading ? (
                  <>
                    <svg
                      className="h-5 w-5 animate-spin text-ink"
                      xmlns="http://www.w3.org/2000/svg"
                      fill="none"
                      viewBox="0 0 24 24"
                      aria-hidden="true"
                    >
                      <circle
                        className="opacity-25"
                        cx="12"
                        cy="12"
                        r="10"
                        stroke="currentColor"
                        strokeWidth="4"
                      />
                      <path
                        className="opacity-75"
                        fill="currentColor"
                        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                      />
                    </svg>
                    Processing…
                  </>
                ) : (
                  "Request test DEW"
                )}
              </button>
            </form>
          </div>

          <div className="mt-8 grid grid-cols-1 gap-3 sm:grid-cols-3">
            <div className="stat-chip">
              <p className="font-mono text-[10px] tracking-wider text-muted uppercase">Drip amount</p>
              <p className="mt-1.5 font-display text-lg font-bold tracking-tight text-frost">
                {infoLoading ? "…" : formatDewAmount(info?.amountWei || "1000000000000000000")}
              </p>
            </div>
            <div className="stat-chip">
              <p className="font-mono text-[10px] tracking-wider text-muted uppercase">
                Limit / address
              </p>
              <p className="mt-1.5 text-sm font-semibold text-frost">
                {infoLoading
                  ? "…"
                  : `${info?.perAddress} / ${formatSeconds(info?.perAddressWindowSec || 86400)}`}
              </p>
            </div>
            <div className="stat-chip">
              <p className="font-mono text-[10px] tracking-wider text-muted uppercase">Chain ID</p>
              <p className="mt-1.5 font-display text-lg font-bold tracking-tight text-frost">
                {infoLoading ? "…" : info?.chainId}
              </p>
            </div>
          </div>

          {!infoLoading && info?.from && (
            <div className="mt-6 flex flex-col items-center font-mono text-xs text-muted">
              <span className="mb-1.5 tracking-wide uppercase text-[10px]">Faucet address</span>
              <div className="flex items-center gap-2 rounded-lg border border-[var(--color-line)] bg-panel/30 px-3 py-1.5">
                <span className="max-w-[200px] truncate text-slate md:max-w-xs">{info.from}</span>
                <button
                  type="button"
                  onClick={() => copyToClipboard(info.from, "from")}
                  className="btn-ghost !px-2 !py-1"
                  title="Copy address"
                >
                  {copied && copiedField === "from" ? "Copied" : "Copy"}
                </button>
              </div>
            </div>
          )}
        </div>
      </main>

      <footer className="relative z-10 border-t border-[var(--color-line)]">
        <div className="mx-auto flex max-w-6xl flex-col gap-2 px-5 py-6 sm:flex-row sm:items-center sm:justify-between sm:px-8">
          <p className="font-mono text-[11px] text-muted">
            © {year} Dew Network · Apache-2.0
          </p>
          <p className="font-mono text-[11px] text-muted">
            Testnet only — never use production keys here
          </p>
        </div>
      </footer>
    </div>
  );
}
