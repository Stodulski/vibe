/**
 * Sharing a club's public page. The native share sheet comes first. Without one, the link is
 * copied. Without a clipboard, the text and the link go to WhatsApp. The browser is passed in
 * as an environment, so the order of the three can be tested without a browser.
 */

export interface SharePayload {
  title: string;
  text: string;
  url: string;
}

/** `cancelled` means the player closed the share sheet on purpose, so nothing is shown. */
export type ShareOutcome = 'shared' | 'cancelled' | 'copied' | 'whatsapp';

export interface ShareEnvironment {
  /** The native share sheet, when the browser has one. */
  share?: ((data: SharePayload) => Promise<void>) | undefined;
  /** The clipboard write, when the page can reach one. */
  copy?: ((text: string) => Promise<void>) | undefined;
  /** Opens a link in a new tab. */
  openWhatsApp: (href: string) => void;
}

export function whatsappShareHref({ text, url }: SharePayload): string {
  return `https://wa.me/?text=${encodeURIComponent(`${text} ${url}`)}`;
}

function isAbortError(error: unknown): boolean {
  return typeof error === 'object' && error !== null && 'name' in error && error.name === 'AbortError';
}

export async function shareComplexPage(payload: SharePayload, env: ShareEnvironment): Promise<ShareOutcome> {
  if (env.share) {
    try {
      await env.share({ title: payload.title, text: payload.text, url: payload.url });
      return 'shared';
    } catch (error) {
      if (isAbortError(error)) return 'cancelled';
      // Any other failure of the native sheet falls through to the clipboard.
    }
  }

  try {
    if (!env.copy) throw new Error('clipboard unavailable');
    await env.copy(payload.url);
    return 'copied';
  } catch {
    env.openWhatsApp(whatsappShareHref(payload));
    return 'whatsapp';
  }
}

/** The environment of the current browser. Call it on click, never during render. */
export function browserShareEnvironment(): ShareEnvironment {
  // Optional by contract: a page on an insecure origin has no clipboard at all.
  const { clipboard } = navigator as Partial<Navigator>;
  return {
    share: typeof navigator.share === 'function' ? (data) => navigator.share(data) : undefined,
    copy: clipboard ? (text) => clipboard.writeText(text) : undefined,
    openWhatsApp: (href) => {
      window.open(href, '_blank', 'noopener,noreferrer');
    },
  };
}
