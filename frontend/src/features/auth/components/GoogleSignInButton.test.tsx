import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, render, screen, waitFor } from '@testing-library/react';
import type { GoogleIdConfiguration, GoogleButtonConfiguration, GoogleNamespace } from '@/shared/lib/googleIdentity';
import { ES_AR } from '@/shared/i18n/es_AR';
import { GIS_BUTTON_HEIGHT, GIS_BUTTON_WIDTH, GoogleSignInButton } from './GoogleSignInButton';

const mockMutate = vi.fn();
vi.mock('../hooks/useGoogleSignIn', () => ({
  useGoogleSignIn: () => ({ mutate: mockMutate }),
}));

const mockEnv = vi.hoisted((): { VITE_GOOGLE_CLIENT_ID: string | undefined } => ({
  VITE_GOOGLE_CLIENT_ID: 'test-client-id',
}));
vi.mock('@/shared/lib/env', () => ({ env: mockEnv }));

let initializeConfig: GoogleIdConfiguration | null = null;
let renderButtonOptions: GoogleButtonConfiguration | null = null;
let renderButtonParent: HTMLElement | null = null;
let loadResult: Promise<GoogleNamespace> = Promise.resolve() as never;

vi.mock('@/shared/lib/googleIdentity', () => ({
  loadGoogleIdentityServices: () => loadResult,
}));

function stubGoogleNamespace(): GoogleNamespace {
  return {
    accounts: {
      id: {
        initialize: vi.fn((config: GoogleIdConfiguration) => {
          initializeConfig = config;
        }),
        renderButton: vi.fn((parent: HTMLElement, options: GoogleButtonConfiguration) => {
          renderButtonParent = parent;
          renderButtonOptions = options;
        }),
      },
    },
  };
}

describe('GoogleSignInButton — rendering', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockEnv.VITE_GOOGLE_CLIENT_ID = 'test-client-id';
    initializeConfig = null;
    renderButtonOptions = null;
    renderButtonParent = null;
  });

  it('renders nothing when no client id is configured', () => {
    mockEnv.VITE_GOOGLE_CLIENT_ID = undefined;

    const { container } = render(<GoogleSignInButton />);

    expect(container).toBeEmptyDOMElement();
  });

  it('initializes Google Identity Services with the configured client id', async () => {
    const google = stubGoogleNamespace();
    loadResult = Promise.resolve(google);

    render(<GoogleSignInButton />);

    await waitFor(() => {
      expect(initializeConfig).not.toBeNull();
    });

    expect(initializeConfig).toMatchObject({
      client_id: 'test-client-id',
      ux_mode: 'popup',
      auto_select: false,
      use_fedcm_for_prompt: true,
    });
    expect(typeof initializeConfig?.callback).toBe('function');
  });

  it('renders the official button with the expected look and locale', async () => {
    const google = stubGoogleNamespace();
    loadResult = Promise.resolve(google);

    render(<GoogleSignInButton />);

    await waitFor(() => {
      expect(renderButtonOptions).not.toBeNull();
    });

    expect(renderButtonOptions).toMatchObject({
      theme: 'outline',
      size: 'large',
      text: 'continue_with',
      locale: 'es',
    });
    expect(renderButtonOptions?.width).toBe(GIS_BUTTON_WIDTH);
    expect(renderButtonParent).toBe(screen.getByTestId('google-button-host'));
  });
});

// A sibling describe, not nested: max-lines-per-function counts a describe
// callback's whole body.
describe('GoogleSignInButton — shape', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockEnv.VITE_GOOGLE_CLIENT_ID = 'test-client-id';
    renderButtonParent = null;
  });

  // Google's button cannot be drawn at the primary button's size, so the one
  // the visitor sees is a decoy and Google's own is the invisible layer that
  // takes every click. Both halves matter: a visible decoy with no covering
  // layer is a button that does nothing.
  it('shows a decoy shaped like the primary button under an invisible Google layer', async () => {
    loadResult = Promise.resolve(stubGoogleNamespace());

    render(<GoogleSignInButton />);

    const decoy = screen.getByText(ES_AR.auth.continueWithGoogle);
    expect(decoy).toHaveAttribute('aria-hidden', 'true');
    expect(decoy).toHaveClass('rounded-full');
    const host = screen.getByTestId('google-button-host');
    expect(host).toHaveClass('opacity-0');
    expect(host).toHaveStyle({ width: `${String(GIS_BUTTON_WIDTH)}px`, height: `${String(GIS_BUTTON_HEIGHT)}px` });
    await waitFor(() => {
      expect(renderButtonParent).toBe(host);
    });
  });

  // Does not enable One Tap — auto_select is asserted above, and there is
  // no separate `prompt()` call anywhere in the component to assert on.
  it('never calls a One Tap prompt (no such API is exposed by the stub)', async () => {
    const google = stubGoogleNamespace();
    loadResult = Promise.resolve(google);

    render(<GoogleSignInButton />);

    await waitFor(() => {
      expect(initializeConfig).not.toBeNull();
    });

    expect(Object.keys(google.accounts.id)).toEqual(['initialize', 'renderButton']);
  });
});

describe('GoogleSignInButton — behavior', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockEnv.VITE_GOOGLE_CLIENT_ID = 'test-client-id';
    initializeConfig = null;
    renderButtonOptions = null;
    renderButtonParent = null;
  });

  it('forwards the credential from the callback to useGoogleSignIn', async () => {
    const google = stubGoogleNamespace();
    loadResult = Promise.resolve(google);

    render(<GoogleSignInButton />);

    await waitFor(() => {
      expect(initializeConfig).not.toBeNull();
    });

    act(() => {
      initializeConfig?.callback({ credential: 'google-credential-token' });
    });

    expect(mockMutate).toHaveBeenCalledWith('google-credential-token');
  });

  it('shows a plain fallback message when the script fails to load', async () => {
    loadResult = Promise.reject(new Error('script failed'));

    render(<GoogleSignInButton />);

    await waitFor(() => {
      expect(screen.getByText(ES_AR.auth.googleUnavailable)).toBeInTheDocument();
    });
  });
});
