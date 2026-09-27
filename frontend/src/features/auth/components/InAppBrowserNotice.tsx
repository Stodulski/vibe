import { useEffect, useRef, useState } from 'react';
import { Copy, Check } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { detectInAppBrowser } from '../lib/inAppBrowser';

const t = ES_AR;

/**
 * Builds the Android "open in Chrome" intent URL for the current page, so
 * the visitor lands on the exact same destination in a real browser instead
 * of Chrome's homepage.
 */
function buildChromeIntentUrl(location: Pick<Location, 'host' | 'pathname' | 'search'>): string {
  return `intent://${location.host}${location.pathname}${location.search}#Intent;scheme=https;package=com.android.chrome;end`;
}

/**
 * `navigator.clipboard` per TypeScript's DOM lib is always defined, but at
 * runtime it is missing in some embedding webviews — `Partial` makes the
 * absence visible to the type checker instead of asserting it away.
 */
function getClipboard(): Clipboard | undefined {
  return (navigator as Partial<Navigator>).clipboard;
}

/**
 * Replaces the Google button in an in-app browser (see `GoogleSignInSection`
 * and `inAppBrowser.ts`), because Google's OAuth endpoint refuses to work
 * there. Email/password sign-in is unaffected and stays visible above this.
 *
 * Gives the visitor a way out: an "Abrir en Chrome" intent link on Android
 * (which WebView-hosting apps like Instagram and Facebook honor), generic
 * "open in your browser" instructions on iOS (there is no single menu
 * across every host app), and a copy-link fallback that works everywhere.
 */
export function InAppBrowserNotice() {
  const [copied, setCopied] = useState(false);
  const copiedTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (copiedTimeoutRef.current) clearTimeout(copiedTimeoutRef.current);
    },
    [],
  );

  const { platform } = detectInAppBrowser(navigator.userAgent);

  function handleCopy() {
    getClipboard()
      ?.writeText(window.location.href)
      .then(() => {
        setCopied(true);
        if (copiedTimeoutRef.current) clearTimeout(copiedTimeoutRef.current);
        copiedTimeoutRef.current = setTimeout(() => {
          setCopied(false);
        }, 2000);
      })
      .catch(() => {
        // The host app denied the clipboard permission, or there is none to
        // grant — nothing else to do here; the button just stays as it was
        // so the visitor can try again or copy the address bar manually.
      });
  }

  return (
    <div
      data-testid="in-app-browser-notice"
      className="border-border-subtle bg-bg-elevated space-y-3 rounded-2xl border p-4"
    >
      <p className="text-text-secondary text-sm">{t.auth.inAppBrowserExplain}</p>

      {platform === 'android' && (
        <a
          href={buildChromeIntentUrl(window.location)}
          className="border-border-interactive bg-background text-foreground hover:border-border-interactive-hover hover:bg-accent focus-visible:ring-ring/50 flex h-11 w-full items-center justify-center rounded-full border text-sm font-semibold transition-colors focus-visible:ring-[3px] focus-visible:outline-none"
        >
          {t.auth.inAppBrowserOpenInChrome}
        </a>
      )}

      {platform === 'ios' && <p className="text-text-tertiary text-sm">{t.auth.inAppBrowserOpenIOSInstruction}</p>}

      <button
        type="button"
        onClick={handleCopy}
        className="border-border-subtle bg-bg-base text-primary-400 hover:bg-bg-overlay inline-flex h-8 w-full items-center justify-center gap-1.5 rounded-lg border px-2.5 text-sm font-medium transition-colors"
      >
        {copied ? <Check className="text-success-text size-3.5" /> : <Copy className="text-primary-400 size-3.5" />}
        <span aria-live="polite">{copied ? t.auth.inAppBrowserLinkCopied : t.auth.inAppBrowserCopyLink}</span>
      </button>
    </div>
  );
}
