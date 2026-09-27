import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider, type RouteObject } from 'react-router-dom';
import type { GoogleNamespace } from '@/shared/lib/googleIdentity';
import { ES_AR } from '@/shared/i18n/es_AR';
import { GoogleSignInSection } from './GoogleSignInSection';

const mockEnv = vi.hoisted((): { VITE_GOOGLE_CLIENT_ID: string | undefined } => ({
  VITE_GOOGLE_CLIENT_ID: 'test-client-id',
}));
vi.mock('@/shared/lib/env', () => ({ env: mockEnv }));

// The section renders the real GoogleSignInButton in the "not in-app" case,
// which loads Google's script — stub it exactly like GoogleSignInButton's
// own test does, so this file stays about routing to the right child, not
// about Google Identity Services itself.
const stubGoogleNamespace: GoogleNamespace = {
  accounts: {
    id: {
      initialize: vi.fn(),
      renderButton: vi.fn(),
    },
  },
};
vi.mock('@/shared/lib/googleIdentity', () => ({
  loadGoogleIdentityServices: () => Promise.resolve(stubGoogleNamespace),
}));

const INSTAGRAM_ANDROID_UA =
  'Mozilla/5.0 (Linux; Android 10; SM-G960F Build/QP1A.190711.020; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/96.0.4664.104 Mobile Safari/537.36 Instagram 224.0.0.15.109 Android';
const INSTAGRAM_IOS_UA =
  'Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Instagram 224.0.0.15.109 (iPhone12,1; iOS 15_0)';
const CHROME_ANDROID_UA =
  'Mozilla/5.0 (Linux; Android 12; Pixel 6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.127 Mobile Safari/537.36';

function stubUserAgent(ua: string) {
  return vi.spyOn(window.navigator, 'userAgent', 'get').mockReturnValue(ua);
}

function renderSection() {
  const routes: RouteObject[] = [{ path: '*', element: <GoogleSignInSection /> }];
  const router = createMemoryRouter(routes, { initialEntries: ['/login'] });
  return render(<RouterProvider router={router} />);
}

describe('GoogleSignInSection — in-app browser routing', () => {
  let userAgentSpy: ReturnType<typeof stubUserAgent> | undefined;

  beforeEach(() => {
    vi.clearAllMocks();
    mockEnv.VITE_GOOGLE_CLIENT_ID = 'test-client-id';
  });

  afterEach(() => {
    userAgentSpy?.mockRestore();
  });

  it('shows the in-app browser notice instead of the Google button inside Instagram', () => {
    userAgentSpy = stubUserAgent(INSTAGRAM_ANDROID_UA);

    renderSection();

    expect(screen.getByTestId('in-app-browser-notice')).toBeInTheDocument();
    expect(screen.queryByTestId('google-button-host')).not.toBeInTheDocument();
  });

  it('renders the Android intent link to open Chrome inside an in-app browser on Android', () => {
    userAgentSpy = stubUserAgent(INSTAGRAM_ANDROID_UA);

    renderSection();

    const openInChrome = screen.getByRole('link', { name: ES_AR.auth.inAppBrowserOpenInChrome });
    expect(openInChrome).toHaveAttribute('href', expect.stringContaining('intent://'));
    expect(openInChrome).toHaveAttribute('href', expect.stringContaining('package=com.android.chrome'));
  });

  it('shows generic open-in-browser instructions instead of an intent link on iOS', () => {
    userAgentSpy = stubUserAgent(INSTAGRAM_IOS_UA);

    renderSection();

    expect(screen.getByText(ES_AR.auth.inAppBrowserOpenIOSInstruction)).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: ES_AR.auth.inAppBrowserOpenInChrome })).not.toBeInTheDocument();
  });

  it('renders the real Google button outside an in-app browser', async () => {
    userAgentSpy = stubUserAgent(CHROME_ANDROID_UA);

    renderSection();

    await waitFor(() => {
      expect(screen.getByTestId('google-button-host')).toBeInTheDocument();
    });
    expect(screen.queryByTestId('in-app-browser-notice')).not.toBeInTheDocument();
  });
});

describe('GoogleSignInSection — copying the link from the notice', () => {
  let userAgentSpy: ReturnType<typeof stubUserAgent> | undefined;
  let writeText: Mock<(data: string) => Promise<void>>;

  beforeEach(() => {
    vi.clearAllMocks();
    mockEnv.VITE_GOOGLE_CLIENT_ID = 'test-client-id';
    userAgentSpy = stubUserAgent(INSTAGRAM_ANDROID_UA);
    writeText = vi.fn<(data: string) => Promise<void>>().mockResolvedValue(undefined);
  });

  afterEach(() => {
    userAgentSpy?.mockRestore();
    vi.restoreAllMocks();
  });

  // `userEvent.setup()` installs its own clipboard stub on `navigator`, so
  // the fake must be installed after it (and after render, to be safe) —
  // installing it any earlier gets silently overwritten.
  function stubClipboard(value: Pick<Clipboard, 'writeText'> | undefined) {
    Object.defineProperty(navigator, 'clipboard', { value, configurable: true });
  }

  it('copies the current page URL and shows a brief confirmation', async () => {
    const user = userEvent.setup();
    renderSection();
    stubClipboard({ writeText });

    await user.click(screen.getByRole('button', { name: ES_AR.auth.inAppBrowserCopyLink }));

    expect(writeText).toHaveBeenCalledWith(window.location.href);
    expect(await screen.findByText(ES_AR.auth.inAppBrowserLinkCopied)).toBeInTheDocument();
  });

  it('does nothing and does not crash when the clipboard refuses the write', async () => {
    writeText.mockRejectedValueOnce(new Error('denied'));
    const user = userEvent.setup();
    renderSection();
    stubClipboard({ writeText });

    await user.click(screen.getByRole('button', { name: ES_AR.auth.inAppBrowserCopyLink }));

    await waitFor(() => {
      expect(writeText).toHaveBeenCalled();
    });
    expect(screen.queryByText(ES_AR.auth.inAppBrowserLinkCopied)).not.toBeInTheDocument();
  });

  it('does nothing and does not crash when no clipboard API exists', async () => {
    const user = userEvent.setup();
    renderSection();
    stubClipboard(undefined);

    await user.click(screen.getByRole('button', { name: ES_AR.auth.inAppBrowserCopyLink }));

    expect(screen.queryByText(ES_AR.auth.inAppBrowserLinkCopied)).not.toBeInTheDocument();
  });
});
