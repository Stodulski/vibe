import { createRef, act } from 'react';
import { render, screen } from '@testing-library/react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { TurnstileField, type TurnstileFieldHandle } from './TurnstileField';

interface CapturedTurnstileProps {
  siteKey: string;
  options?: { theme?: string; size?: string; language?: string; appearance?: string };
  onSuccess?: (token: string) => void;
  onExpire?: () => void;
  onError?: () => void;
}

let turnstileProps: CapturedTurnstileProps | null = null;

// The real widget injects Cloudflare's script and talks to their API — none
// of which exists in a test environment. This mock captures the props
// `<Turnstile>` receives so a test can drive its callbacks directly.
vi.mock('@marsidev/react-turnstile', () => ({
  Turnstile: vi.fn((props: CapturedTurnstileProps) => {
    turnstileProps = props;
    return <div data-testid="turnstile-widget" data-site-key={props.siteKey} />;
  }),
}));

// A mutable object (not a factory return value) so individual tests can flip
// `VITE_TURNSTILE_SITE_KEY` and have the next render see the new value —
// `TurnstileField` reads `env.VITE_TURNSTILE_SITE_KEY` at render time, not at
// import time. `vi.hoisted` so it exists by the time the hoisted `vi.mock`
// factory below runs.
const mockEnv = vi.hoisted((): { VITE_TURNSTILE_SITE_KEY: string | undefined } => ({
  VITE_TURNSTILE_SITE_KEY: 'test-site-key',
}));
vi.mock('@/shared/lib/env', () => ({ env: mockEnv }));

beforeEach(() => {
  vi.clearAllMocks();
  turnstileProps = null;
  mockEnv.VITE_TURNSTILE_SITE_KEY = 'test-site-key';
});

describe('TurnstileField', () => {
  it('renders nothing when no site key is configured', () => {
    mockEnv.VITE_TURNSTILE_SITE_KEY = undefined;

    render(<TurnstileField onTokenChange={vi.fn()} />);

    expect(screen.queryByTestId('turnstile-widget')).not.toBeInTheDocument();
  });

  it('renders the widget with the configured site key when a site key exists', () => {
    render(<TurnstileField onTokenChange={vi.fn()} />);

    expect(screen.getByTestId('turnstile-widget')).toHaveAttribute('data-site-key', 'test-site-key');
  });

  it('calls onTokenChange with the solved token on success', () => {
    const onTokenChange = vi.fn();

    render(<TurnstileField onTokenChange={onTokenChange} />);
    act(() => {
      turnstileProps?.onSuccess?.('solved-token');
    });

    expect(onTokenChange).toHaveBeenCalledWith('solved-token');
  });

  it('clears the token when the challenge expires', () => {
    const onTokenChange = vi.fn();

    render(<TurnstileField onTokenChange={onTokenChange} />);
    act(() => {
      turnstileProps?.onSuccess?.('solved-token');
      turnstileProps?.onExpire?.();
    });

    expect(onTokenChange).toHaveBeenLastCalledWith(undefined);
  });

  it('clears the token and shows the aria-live error message when the widget errors', () => {
    const onTokenChange = vi.fn();

    render(<TurnstileField onTokenChange={onTokenChange} />);
    act(() => {
      turnstileProps?.onError?.();
    });

    expect(onTokenChange).toHaveBeenLastCalledWith(undefined);
    expect(screen.getByText(ES_AR.auth.turnstileChallengeError)).toBeInTheDocument();
  });

  it('clears the token via the exposed reset() and hides the error message', () => {
    const onTokenChange = vi.fn();
    const ref = createRef<TurnstileFieldHandle>();

    render(<TurnstileField ref={ref} onTokenChange={onTokenChange} />);
    act(() => {
      turnstileProps?.onError?.();
    });
    expect(screen.getByText(ES_AR.auth.turnstileChallengeError)).toBeInTheDocument();

    act(() => {
      ref.current?.reset();
    });

    expect(onTokenChange).toHaveBeenLastCalledWith(undefined);
    expect(screen.queryByText(ES_AR.auth.turnstileChallengeError)).not.toBeInTheDocument();
  });
});

// A sibling describe, not nested: max-lines-per-function counts a describe
// callback's whole body.
describe('TurnstileField visibility', () => {
  it('asks Cloudflare to show the widget only when an interaction is required', () => {
    render(<TurnstileField onTokenChange={vi.fn()} />);

    expect(turnstileProps?.options?.appearance).toBe('interaction-only');
  });

  it('collapses the widget once solved and brings it back when the token expires', () => {
    render(<TurnstileField onTokenChange={vi.fn()} />);
    const slot = screen.getByTestId('turnstile-slot');
    expect(slot).not.toHaveAttribute('hidden');

    act(() => {
      turnstileProps?.onSuccess?.('solved-token');
    });
    expect(slot).toHaveAttribute('hidden');
    // Hidden, not unmounted: the widget keeps its own expiry/refresh cycle.
    expect(screen.getByTestId('turnstile-widget')).toBeInTheDocument();

    act(() => {
      turnstileProps?.onExpire?.();
    });
    expect(slot).not.toHaveAttribute('hidden');
  });

  it('brings the widget back after reset() so a failed submit can be retried', () => {
    const ref = createRef<TurnstileFieldHandle>();

    render(<TurnstileField ref={ref} onTokenChange={vi.fn()} />);
    act(() => {
      turnstileProps?.onSuccess?.('solved-token');
    });
    expect(screen.getByTestId('turnstile-slot')).toHaveAttribute('hidden');

    act(() => {
      ref.current?.reset();
    });
    expect(screen.getByTestId('turnstile-slot')).not.toHaveAttribute('hidden');
  });
});
