import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { toast } from 'sonner';
import { server } from '../../test/msw/server';
import { renderBooking } from '../../test/booking';
import { EVENT } from '../../test/eventFixtures';
import type { BookingConfig } from '../../config';
import { BOOKING_MESSAGES } from '../../messages';
import { EventSignUpForm } from './EventSignUpForm';

const t = BOOKING_MESSAGES.publicBooking;

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() } }));
vi.mock('../../lib/navigation', () => ({ navigateTo: vi.fn() }));

const turnstile = vi.hoisted(() => ({
  onSuccess: undefined as ((token: string) => void) | undefined,
  mounts: { count: 0 },
}));

// A stand-in widget: a test solves the challenge through `onSuccess`, and a
// fresh token means a fresh mount.
vi.mock('@marsidev/react-turnstile', async () => {
  const { useEffect } = await import('react');
  return {
    Turnstile: (props: { onSuccess?: (token: string) => void }) => {
      turnstile.onSuccess = props.onSuccess;
      useEffect(() => {
        turnstile.mounts.count += 1;
      }, []);
      return <div data-testid="turnstile-widget" />;
    },
  };
});

const BASE_CONFIG: BookingConfig = {
  apiBaseUrl: 'http://localhost/api/v1',
  publicSiteUrl: 'http://localhost',
  privacyUrl: 'https://vibe.com.ar/privacidad',
};
const TURNSTILE_CONFIG: BookingConfig = { ...BASE_CONFIG, turnstileSiteKey: 'test-site-key' };

function captureSignUp(status = 201, body: Record<string, unknown> = {}) {
  const bodies: Record<string, unknown>[] = [];
  server.use(
    http.post('*/public/complexes/test-club/events/ev1/registrations', async ({ request }) => {
      bodies.push((await request.json()) as Record<string, unknown>);
      if (status !== 201) return HttpResponse.json(body, { status });
      return HttpResponse.json({ token: 'tok', mp_init_point: 'https://mp.test/pay' }, { status });
    }),
  );
  return bodies;
}

async function fillAndPay(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText(/nombre/i), 'Ana');
  await user.type(screen.getByLabelText(/apellido/i), 'Diaz');
  await user.type(screen.getByLabelText(/tel.fono/i), '1123456789');
  await user.click(screen.getByRole('button', { name: /pagar/i }));
}

function resetTurnstile() {
  turnstile.onSuccess = undefined;
  turnstile.mounts.count = 0;
  vi.mocked(toast.info).mockClear();
}

describe('EventSignUpForm Turnstile', () => {
  beforeEach(resetTurnstile);

  it('sends the solved token with the sign-up, like the booking hold', async () => {
    const bodies = captureSignUp();
    const user = userEvent.setup();
    renderBooking(<EventSignUpForm slug="test-club" event={EVENT} />, { config: TURNSTILE_CONFIG });
    act(() => {
      turnstile.onSuccess?.('solved-token');
    });

    await fillAndPay(user);

    await waitFor(() => {
      expect(bodies).toHaveLength(1);
    });
    expect(bodies[0]).toMatchObject({ first_name: 'Ana', turnstile_token: 'solved-token' });
  });

  it('waits for the challenge when it is not solved yet, and signs up once it is', async () => {
    const bodies = captureSignUp();
    const user = userEvent.setup();
    renderBooking(<EventSignUpForm slug="test-club" event={EVENT} />, { config: TURNSTILE_CONFIG });

    await fillAndPay(user);
    expect(toast.info).toHaveBeenCalledWith(t.holdChallengePending);
    expect(bodies).toHaveLength(0);

    act(() => {
      turnstile.onSuccess?.('late-token');
    });
    await waitFor(() => {
      expect(bodies).toHaveLength(1);
    });
    expect(bodies[0]).toMatchObject({ turnstile_token: 'late-token' });
  });
});

describe('EventSignUpForm Turnstile refusal', () => {
  beforeEach(resetTurnstile);

  it('shows the failure message and asks for a fresh token when the server refuses it', async () => {
    captureSignUp(422, {
      type: 'https://vibe.com.ar/problems/validation',
      title: 'Validation failed',
      status: 422,
      errors: [{ field: 'turnstile_token', message: 'invalid' }],
    });
    const user = userEvent.setup();
    renderBooking(<EventSignUpForm slug="test-club" event={EVENT} />, { config: TURNSTILE_CONFIG });
    act(() => {
      turnstile.onSuccess?.('spent-token');
    });

    await fillAndPay(user);

    expect(await screen.findByText(t.turnstileInvalid)).toBeInTheDocument();
    await waitFor(() => {
      expect(turnstile.mounts.count).toBe(2);
    });
  });

  it('signs up without a token when the storefront has no site key', async () => {
    const bodies = captureSignUp();
    const user = userEvent.setup();
    renderBooking(<EventSignUpForm slug="test-club" event={EVENT} />, { config: BASE_CONFIG });

    await fillAndPay(user);

    await waitFor(() => {
      expect(bodies).toHaveLength(1);
    });
    expect(bodies[0]).not.toHaveProperty('turnstile_token');
    expect(screen.queryByTestId('turnstile-widget')).not.toBeInTheDocument();
  });
});
