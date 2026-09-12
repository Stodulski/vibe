import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
import { LoginForm } from './LoginForm';
import { renderWithProviders } from '@/test/test-utils';

vi.mock('../hooks/useLogin', () => ({
  useLogin: () => ({ mutate: vi.fn(), isPending: false, isError: false, error: null }),
}));

vi.mock('../hooks/useGoogleSignIn', () => ({
  useGoogleSignIn: () => ({ mutate: vi.fn() }),
}));

// Never resolves — these tests only care about whether the section renders
// at all, not about the widget itself (see GoogleSignInButton.test.tsx).
vi.mock('@/shared/lib/googleIdentity', () => ({
  loadGoogleIdentityServices: () => new Promise(() => undefined),
}));

const mockEnv = vi.hoisted(
  (): { VITE_GOOGLE_CLIENT_ID: string | undefined; VITE_TURNSTILE_SITE_KEY: string | undefined } => ({
    VITE_GOOGLE_CLIENT_ID: undefined,
    VITE_TURNSTILE_SITE_KEY: undefined,
  }),
);
vi.mock('@/shared/lib/env', () => ({ env: mockEnv }));

describe('LoginForm — Google sign-in section', () => {
  beforeEach(() => {
    mockEnv.VITE_GOOGLE_CLIENT_ID = undefined;
  });

  it('renders neither the divider nor the Google button when no client id is configured', () => {
    renderWithProviders(<LoginForm />);
    expect(screen.queryByRole('separator')).not.toBeInTheDocument();
  });

  it('renders the divider and the Google button below the submit button when a client id is configured', () => {
    mockEnv.VITE_GOOGLE_CLIENT_ID = 'test-client-id';

    renderWithProviders(<LoginForm />);

    expect(screen.getByRole('separator')).toBeInTheDocument();
  });
});
