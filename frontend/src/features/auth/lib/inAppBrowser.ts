/**
 * Detects in-app browsers (Instagram, Facebook, Messenger, LINE, TikTok, and
 * generic Android WebView) from a user agent string.
 *
 * Google blocks its OAuth authorization endpoint inside these embedded
 * webviews with `Error 403: disallowed_useragent` — see
 * https://developers.googleblog.com/upcoming-security-changes-to-googles-oauth-20-authorization-endpoint-in-embedded-webviews/.
 * Many visitors arrive here from an Instagram or Facebook link, so
 * "Continuar con Google" can never work for them. This module only answers
 * "which in-app browser is this", never touches the sign-in flow itself —
 * see `InAppBrowserNotice` for what replaces the Google button when it
 * returns a match.
 */

/** The in-app browsers this app knows how to detect. */
export type InAppBrowserApp = 'instagram' | 'facebook' | 'messenger' | 'line' | 'tiktok' | 'android-webview';

/** The OS family, used to pick the right escape hatch (intent link vs. menu instructions). */
type InAppBrowserPlatform = 'android' | 'ios' | 'other';

export interface InAppBrowserInfo {
  /** The detected in-app browser, or `null` when the user agent looks like a real browser. */
  app: InAppBrowserApp | null;
  platform: InAppBrowserPlatform;
}

// Messenger ships its own markers on both platforms and is called out
// separately in the scope, so it is matched before the broader Facebook
// family patterns below.
const MESSENGER_PATTERN = /FBAN\/MessengerForiOS|FBAN\/MessengerForAndroid|FB_IAB\/MESSENGER/i;

// The Facebook app's in-app browser (and Facebook Lite) tag their webview
// with one or more of these tokens, embedded in the UA's own comment or
// bracket section rather than replacing it.
const FACEBOOK_PATTERN = /FBAN|FBAV|FB_IAB|FBIOS/;

const INSTAGRAM_MARKER = 'Instagram';

// LINE's in-app browser appends "Line/<version>" to the platform's ordinary
// browser UA.
const LINE_MARKER = 'Line/';

// TikTok's webview identifies itself with any of these, depending on version
// and platform.
const TIKTOK_PATTERN = /musical_ly|BytedanceWebview|TikTok/i;

// A generic Android WebView (any app embedding one, not just the named ones
// above) appends "; wv)" to the Chrome-derived UA right before the closing
// paren of the platform section. Real Chrome for Android, and Chrome Custom
// Tabs, never carry this marker.
const ANDROID_WEBVIEW_PATTERN = /; ?wv\)/i;

const IOS_PATTERN = /iPhone|iPad|iPod/;
const ANDROID_MARKER = 'Android';

function detectPlatform(userAgent: string): InAppBrowserPlatform {
  if (IOS_PATTERN.test(userAgent)) return 'ios';
  if (userAgent.includes(ANDROID_MARKER)) return 'android';
  return 'other';
}

/**
 * Classifies a user agent string as one of the known in-app browsers, or
 * `null` when it looks like an ordinary browser (Chrome, Safari, Chrome iOS,
 * Firefox iOS, Samsung Internet, any desktop browser, ...).
 *
 * Order matters: the named apps are checked before the generic Android
 * WebView marker, since Instagram/Facebook/etc. on Android also carry
 * `; wv)` and would otherwise be reported as an anonymous webview instead of
 * by name.
 */
export function detectInAppBrowser(userAgent: string): InAppBrowserInfo {
  const platform = detectPlatform(userAgent);

  if (userAgent.includes(INSTAGRAM_MARKER)) return { app: 'instagram', platform };
  if (MESSENGER_PATTERN.test(userAgent)) return { app: 'messenger', platform };
  if (FACEBOOK_PATTERN.test(userAgent)) return { app: 'facebook', platform };
  if (userAgent.includes(LINE_MARKER)) return { app: 'line', platform };
  if (TIKTOK_PATTERN.test(userAgent)) return { app: 'tiktok', platform };
  if (platform === 'android' && ANDROID_WEBVIEW_PATTERN.test(userAgent)) return { app: 'android-webview', platform };

  return { app: null, platform };
}

/** Convenience predicate over {@link detectInAppBrowser} for callers that only need a boolean. */
export function isInAppBrowser(userAgent: string): boolean {
  return detectInAppBrowser(userAgent).app !== null;
}
