import { describe, it, expect } from 'vitest';
import { detectInAppBrowser, isInAppBrowser, type InAppBrowserApp } from './inAppBrowser';

// Real-world user agent strings, not hand-shortened stand-ins: the whole
// point of this table is to prove the patterns survive the noise every
// in-app browser (and every real one) actually ships around its markers.
const IN_APP_CASES: { name: string; ua: string; app: InAppBrowserApp; platform: 'android' | 'ios' }[] = [
  {
    name: 'Instagram on Android',
    ua: 'Mozilla/5.0 (Linux; Android 10; SM-G960F Build/QP1A.190711.020; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/96.0.4664.104 Mobile Safari/537.36 Instagram 224.0.0.15.109 Android (29/10; 420dpi; 1080x2129; samsung; SM-G960F; starlte; samsungexynos8895; en_US; 351899693)',
    app: 'instagram',
    platform: 'android',
  },
  {
    name: 'Instagram on iOS',
    ua: 'Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Instagram 224.0.0.15.109 (iPhone12,1; iOS 15_0; en_US; en-US; scale=2.00; 828x1792; 351899693)',
    app: 'instagram',
    platform: 'ios',
  },
  {
    name: 'Facebook on Android',
    ua: 'Mozilla/5.0 (Linux; Android 10; SM-G960F Build/QP1A.190711.020; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/96.0.4664.104 Mobile Safari/537.36 [FB_IAB/FB4A;FBAV/330.0.0.16.119;]',
    app: 'facebook',
    platform: 'android',
  },
  {
    name: 'Facebook on iOS',
    ua: 'Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 [FBAN/FBIOS;FBAV/330.0.0.24.119;FBBV/354220256;FBDV/iPhone12,1;FBMD/iPhone;FBSN/iOS;FBSV/15.0;FBSS/2;FBID/phone;FBLC/en_US;FBOP/5]',
    app: 'facebook',
    platform: 'ios',
  },
  {
    name: 'Messenger on iOS',
    ua: 'Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 [FBAN/MessengerForiOS;FBAV/330.0.0.24.119;FBBV/354220256;FBDV/iPhone12,1;FBMD/iPhone;FBSN/iOS;FBSV/15.0;FBSS/2;FBID/phone;FBLC/en_US]',
    app: 'messenger',
    platform: 'ios',
  },
  {
    name: 'Messenger on Android',
    ua: 'Mozilla/5.0 (Linux; Android 10; SM-G960F Build/QP1A.190711.020; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/96.0.4664.104 Mobile Safari/537.36 [FB_IAB/MESSENGER;FBAV/330.0.0.16.119;]',
    app: 'messenger',
    platform: 'android',
  },
  {
    name: 'LINE on Android',
    ua: 'Mozilla/5.0 (Linux; Android 10; SM-G960F Build/QP1A.190711.020; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/96.0.4664.104 Mobile Safari/537.36 Line/11.15.1',
    app: 'line',
    platform: 'android',
  },
  {
    name: 'TikTok on Android',
    ua: 'Mozilla/5.0 (Linux; Android 10; SM-G960F Build/QP1A.190711.020; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/96.0.4664.104 Mobile Safari/537.36 musical_ly_2021401030 JsSdk/1.0 NetType/WIFI Channel/googleplay AppName/musical_ly app_version/24.1.3 ByteLocale/en ByteFullLocale/en Region/US BytedanceWebview/d8a21c6',
    app: 'tiktok',
    platform: 'android',
  },
  {
    name: 'TikTok on iOS',
    ua: 'Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 musical_ly_2021401030 JsSdk/1.0 NetType/WIFI Channel/App Store AppName/musical_ly app_version/24.1.3 ByteLocale/en ByteFullLocale/en Region/US',
    app: 'tiktok',
    platform: 'ios',
  },
  {
    name: 'a generic Android WebView with no named host app',
    ua: 'Mozilla/5.0 (Linux; Android 12; Pixel 6 Build/SP2A.220305.013; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/100.0.4896.127 Mobile Safari/537.36',
    app: 'android-webview',
    platform: 'android',
  },
];

const REAL_BROWSER_CASES: { name: string; ua: string }[] = [
  {
    name: 'Chrome on Android',
    ua: 'Mozilla/5.0 (Linux; Android 12; Pixel 6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.127 Mobile Safari/537.36',
  },
  {
    name: 'Chrome Custom Tabs on Android (same UA as ordinary Chrome — no `; wv)` marker)',
    ua: 'Mozilla/5.0 (Linux; Android 12; Pixel 6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.127 Mobile Safari/537.36',
  },
  {
    name: 'Safari on iOS',
    ua: 'Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1',
  },
  {
    name: 'Chrome on iOS (CriOS)',
    ua: 'Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/96.0.4664.53 Mobile/15E148 Safari/604.1',
  },
  {
    name: 'Firefox on iOS (FxiOS)',
    ua: 'Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/40.1 Mobile/15E148 Safari/605.1.15',
  },
  {
    name: 'Samsung Internet',
    ua: 'Mozilla/5.0 (Linux; Android 12; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/17.0 Chrome/100.0.4896.58 Mobile Safari/537.36',
  },
  {
    name: 'desktop Chrome',
    ua: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.127 Safari/537.36',
  },
  {
    name: 'desktop Firefox',
    ua: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:99.0) Gecko/20100101 Firefox/99.0',
  },
];

describe('detectInAppBrowser — in-app browsers', () => {
  it.each(IN_APP_CASES)('flags $name as $app', ({ ua, app, platform }) => {
    expect(detectInAppBrowser(ua)).toEqual({ app, platform });
  });

  it.each(IN_APP_CASES)('reports $name as an in-app browser via isInAppBrowser', ({ ua }) => {
    expect(isInAppBrowser(ua)).toBe(true);
  });
});

describe('detectInAppBrowser — real browsers', () => {
  it.each(REAL_BROWSER_CASES)('never flags $name', ({ ua }) => {
    expect(detectInAppBrowser(ua).app).toBeNull();
  });

  it.each(REAL_BROWSER_CASES)('reports $name as not an in-app browser via isInAppBrowser', ({ ua }) => {
    expect(isInAppBrowser(ua)).toBe(false);
  });
});

describe('detectInAppBrowser — platform on unmatched user agents', () => {
  it('still reports the android platform for a bare Android UA', () => {
    const chromeAndroid =
      'Mozilla/5.0 (Linux; Android 12; Pixel 6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.127 Mobile Safari/537.36';
    expect(detectInAppBrowser(chromeAndroid).platform).toBe('android');
  });

  it('still reports the ios platform for a bare iOS UA', () => {
    const safariIOS =
      'Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1';
    expect(detectInAppBrowser(safariIOS).platform).toBe('ios');
  });

  it('reports the other platform for a desktop UA', () => {
    const desktopChrome =
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.127 Safari/537.36';
    expect(detectInAppBrowser(desktopChrome).platform).toBe('other');
  });
});
