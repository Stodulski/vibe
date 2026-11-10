import { describe, it, expect, vi, beforeEach, afterEach, type MockInstance } from 'vitest';
import { screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { toast } from 'sonner';
import { renderBooking, setBookingUrl } from '../test/booking';
import { BOOKING_MESSAGES } from '../messages';
import { makeConsumedHttpError } from '../test/factories';
import type { BookingSlotInfo } from '../components/BookingForm';
import type { BookingConfig } from '../config';
import { confirmDraftKey } from '../lib/handoff';
import { STORAGE_KEYS } from '../lib/storageKeys';

const t = BOOKING_MESSAGES;

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const createBooking = vi.fn<(...args: unknown[]) => Promise<unknown>>();
vi.mock('../api/public-booking.api', () => {
  const publicBookingApi = {
    createBooking: (...args: unknown[]) => createBooking(...args),
  };
  return { publicBookingApi, createPublicBookingApi: () => publicBookingApi };
});

interface CapturedTurnstileProps {
  siteKey?: string;
  options?: Record<string, unknown>;
  onSuccess?: (token: string) => void;
  ref?: unknown;
}

const turnstile = vi.hoisted(() => ({
  props: null as CapturedTurnstileProps | null,
  reset: { calls: 0 },
}));

// A stand-in widget: it records its props, so a test can solve the challenge
// with `onSuccess`, and counts `reset()` calls made through its ref.
vi.mock('@marsidev/react-turnstile', async () => {
  const { useImperativeHandle } = await import('react');
  return {
    Turnstile: (props: CapturedTurnstileProps) => {
      turnstile.props = props;
      useImperativeHandle(props.ref as never, () => ({
        reset: () => {
          turnstile.reset.calls += 1;
        },
      }));
      return <div data-testid="turnstile-widget" />;
    },
  };
});

const mockSlotInfo: BookingSlotInfo = {
  complexId: 'c1',
  complexName: 'Club Norte',
  complexPhone: '1155550000',
  courtId: 'ct1',
  courtName: 'Cancha 1',
  date: '2026-03-20',
  startTime: '10:00',
  endTime: '11:30',
  durationMinutes: 90,
  price: 1_000_000,
  depositPercentage: 30,
  cancellationHours: 24,
  sport: 'padel',
};

const BASE_CONFIG: BookingConfig = {
  apiBaseUrl: 'http://localhost/api/v1',
  publicSiteUrl: 'http://localhost',
  privacyUrl: 'https://vibe.com.ar/privacidad',
};
const TURNSTILE_CONFIG: BookingConfig = { ...BASE_CONFIG, turnstileSiteKey: 'test-site-key' };

const SUCCESS = {
  booking: { status: 'pending', collection_status: 'unpaid', refund_status: 'none' },
  token: 'tok-abc123',
};

let replaceSpy: MockInstance<Location['replace']> | undefined;

async function renderPage(config: BookingConfig, { quickBook = false } = {}) {
  const Page = (await import('./BookConfirmPage')).default;
  setBookingUrl('/c/club-norte/book/confirm');
  window.sessionStorage.setItem(confirmDraftKey('club-norte'), JSON.stringify(mockSlotInfo));
  if (quickBook) {
    window.localStorage.setItem(
      STORAGE_KEYS.BOOKING_CLIENT_DATA,
      JSON.stringify({
        client_first_name: 'Juan',
        client_last_name: 'Perez',
        client_phone: '1122334455',
        client_email: 'juan@example.com',
      }),
    );
  }
  renderBooking(<Page slug="club-norte" />, { config });
  await waitFor(() => {
    expect(screen.getByText('Cancha 1')).toBeInTheDocument();
  });
}

function payButton() {
  return screen.getByRole('button', { name: /pagar/i });
}

async function fillFullForm(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText(/nombre/i), 'Juan');
  await user.type(screen.getByLabelText(/apellido/i), 'Perez');
  await user.type(screen.getByLabelText(/tel.fono/i), '1122334455');
  await user.type(screen.getByLabelText(/email/i), 'juan@example.com');
}

async function confirmQuickBookIdentity(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: t.publicBooking.quickBookConfirmYes }));
}

function solveChallenge(token = 'solved-token') {
  act(() => {
    turnstile.props?.onSuccess?.(token);
  });
}

function sentRequest(call = 0): Record<string, unknown> {
  return createBooking.mock.calls[call]?.[0] as Record<string, unknown>;
}

/** Renders the full form, solves the challenge and submits it once. */
async function submitSolvedFullForm() {
  const user = userEvent.setup();
  await renderPage(TURNSTILE_CONFIG);
  await fillFullForm(user);
  solveChallenge();
  await waitFor(() => {
    expect(payButton()).toBeEnabled();
  });
  await user.click(payButton());
}

/** The 422 the API answers when the token check fails, in the shape auth uses. */
function turnstileValidationError(code: string) {
  return makeConsumedHttpError(422, {
    type: 'https://vibe.com.ar/problems/validation',
    title: 'Validation failed',
    status: 422,
    errors: [{ field: 'turnstile_token', message: code }],
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  turnstile.props = null;
  turnstile.reset.calls = 0;
  replaceSpy = vi.spyOn(window.location, 'replace').mockImplementation(() => undefined);
});

afterEach(() => {
  replaceSpy?.mockRestore();
  replaceSpy = undefined;
  setBookingUrl('/');
});

describe('BookConfirmPage without a Turnstile site key', () => {
  it('renders no widget and submits the full form without a turnstile_token', async () => {
    createBooking.mockResolvedValue(SUCCESS);
    const user = userEvent.setup();
    await renderPage(BASE_CONFIG);

    expect(screen.queryByTestId('turnstile-widget')).not.toBeInTheDocument();
    await fillFullForm(user);
    expect(payButton()).toBeEnabled();
    await user.click(payButton());

    await waitFor(() => {
      expect(createBooking).toHaveBeenCalledTimes(1);
    });
    expect(sentRequest()).not.toHaveProperty('turnstile_token');
    await waitFor(() => {
      expect(replaceSpy).toHaveBeenCalledWith('/c/club-norte/book/success');
    });
  });

  it('renders no widget and submits the quick-book path without a turnstile_token', async () => {
    createBooking.mockResolvedValue(SUCCESS);
    const user = userEvent.setup();
    await renderPage(BASE_CONFIG, { quickBook: true });

    await confirmQuickBookIdentity(user);
    expect(screen.queryByTestId('turnstile-widget')).not.toBeInTheDocument();
    await user.click(payButton());

    await waitFor(() => {
      expect(createBooking).toHaveBeenCalledTimes(1);
    });
    expect(sentRequest()).not.toHaveProperty('turnstile_token');
  });
});

describe('BookConfirmPage with a Turnstile site key', () => {
  it('renders the widget with the site key and the owner-app options', async () => {
    const user = userEvent.setup();
    await renderPage(TURNSTILE_CONFIG);
    await fillFullForm(user);

    expect(screen.getByTestId('turnstile-widget')).toBeInTheDocument();
    expect(turnstile.props?.siteKey).toBe('test-site-key');
    expect(turnstile.props?.options).toEqual({
      theme: 'dark',
      size: 'flexible',
      language: 'es',
      appearance: 'interaction-only',
    });
  });

  it('blocks the full-form submit until the challenge is solved, then sends the token', async () => {
    createBooking.mockResolvedValue(SUCCESS);
    const user = userEvent.setup();
    await renderPage(TURNSTILE_CONFIG);
    await fillFullForm(user);

    expect(payButton()).toBeDisabled();
    await user.click(payButton());
    expect(createBooking).not.toHaveBeenCalled();

    solveChallenge();
    await waitFor(() => {
      expect(payButton()).toBeEnabled();
    });
    await user.click(payButton());

    await waitFor(() => {
      expect(createBooking).toHaveBeenCalledTimes(1);
    });
    expect(sentRequest()).toMatchObject({ client_first_name: 'Juan', turnstile_token: 'solved-token' });
  });

  it('blocks the quick-book submit until the challenge is solved, then sends the token', async () => {
    createBooking.mockResolvedValue(SUCCESS);
    const user = userEvent.setup();
    await renderPage(TURNSTILE_CONFIG, { quickBook: true });
    await confirmQuickBookIdentity(user);

    expect(payButton()).toBeDisabled();

    solveChallenge('quick-token');
    await waitFor(() => {
      expect(payButton()).toBeEnabled();
    });
    await user.click(payButton());

    await waitFor(() => {
      expect(createBooking).toHaveBeenCalledTimes(1);
    });
    expect(sentRequest()).toMatchObject({ client_first_name: 'Juan', turnstile_token: 'quick-token' });
  });

});

describe('BookConfirmPage with a Turnstile site key, after an attempt', () => {
  it('keeps the token after a 409 "still in progress", so the retry reuses the same Idempotency-Key', async () => {
    createBooking.mockRejectedValueOnce(
      await makeConsumedHttpError(409, {
        type: 'https://vibe.com.ar/problems/conflict',
        title: 'Conflict',
        status: 409,
        detail: 'a request with this Idempotency-Key is still in progress, retry shortly',
      }),
    );
    await submitSolvedFullForm();

    await waitFor(() => {
      expect(createBooking).toHaveBeenCalledTimes(1);
    });
    // The first request is still running on the server: a fresh token would
    // change the payload, and with it the key, and lose that request's answer.
    expect(turnstile.reset.calls).toBe(0);
  });

  it('resets the widget after an answered attempt, so the next one needs a fresh token', async () => {
    createBooking.mockRejectedValueOnce(await makeConsumedHttpError(500, {}));
    await submitSolvedFullForm();

    await waitFor(() => {
      expect(turnstile.reset.calls).toBe(1);
    });
    // The spent token is gone: submit waits for the next one.
    expect(payButton()).toBeDisabled();
  });

  it('shows a Spanish message and resets the widget on a 422 for turnstile_token', async () => {
    createBooking.mockRejectedValueOnce(
      await turnstileValidationError('invalid'),
    );
    await submitSolvedFullForm();

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(t.publicBooking.turnstileInvalid);
    });
    expect(turnstile.reset.calls).toBe(1);
  });

  it.each([
    ['required', 'turnstileRequired'],
    ['unavailable', 'turnstileUnavailable'],
  ] as const)('maps the %s code to its own Spanish message', async (code, key) => {
    createBooking.mockRejectedValueOnce(
      await turnstileValidationError(code),
    );
    await submitSolvedFullForm();

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(t.publicBooking[key]);
    });
  });
});
