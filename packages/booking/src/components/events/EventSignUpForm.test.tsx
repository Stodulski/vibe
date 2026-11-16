import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { toast } from 'sonner';
import { server } from '../../test/msw/server';
import { renderBooking } from '../../test/booking';
import { EVENT } from '../../test/eventFixtures';
import type { BookingConfig } from '../../config';
import type { PublicOpenPlayEvent } from '../../api/types';
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

describe('EventSignUpForm courts', () => {
  it('names one or two courts on a single line, then each fact once as a chip, like a booking ticket', () => {
    renderBooking(<EventSignUpForm slug="test-club" event={EVENT} />);

    const ticket = screen.getByRole('region', { name: BOOKING_MESSAGES.events.summaryLabel });
    expect(within(ticket).getByText(/Cancha.1, Cancha.2/)).toBeInTheDocument();
    for (const chip of ['Techada', 'Descubierta', 'Césped sintético', 'Con iluminación']) {
      expect(within(ticket).getAllByText(chip)).toHaveLength(1);
    }
  });

  it('counts the courts beyond two instead of naming each one, so the ticket stays short', () => {
    const [base] = EVENT.courts;
    if (!base) throw new Error('the fixture event needs a court');
    const courts = [0, 1, 2].map((i) => ({ ...base, id: `court-${String(i)}`, name: `Cancha ${String(i + 1)}` }));
    renderBooking(<EventSignUpForm slug="test-club" event={{ ...EVENT, courts }} />);

    const ticket = screen.getByRole('region', { name: BOOKING_MESSAGES.events.summaryLabel });
    expect(within(ticket).getByText(BOOKING_MESSAGES.events.courtsCount(3))).toBeInTheDocument();
    expect(within(ticket).queryByText(/Cancha.3/)).not.toBeInTheDocument();
  });

  it('shows no court list when the event has none', () => {
    renderBooking(<EventSignUpForm slug="test-club" event={{ ...EVENT, courts: [], court_names: [] }} />);
    expect(screen.queryByRole('listitem')).not.toBeInTheDocument();
  });
});

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

/** Canchas abiertas de Caballeros, from 6ta to 8va. */
const MEN_6_TO_8: PublicOpenPlayEvent = {
  ...EVENT,
  id: 'ev1',
  modality: 'men',
  level_min: 6,
  level_max: 8,
};

describe('EventSignUpForm declarations', () => {
  beforeEach(resetTurnstile);

  it('titles the event by its characteristics and states its requirement', () => {
    renderBooking(<EventSignUpForm slug="test-club" event={MEN_6_TO_8} />, { config: BASE_CONFIG });

    expect(screen.getByRole('heading', { name: 'Cancha abierta de pádel · Caballeros · 6ta a 8va' })).toBeInTheDocument();
    expect(screen.getByText('Caballeros · de 6ta a 8va')).toBeInTheDocument();
  });

  it('asks the gender and the category only for an event with a modality', () => {
    renderBooking(<EventSignUpForm slug="test-club" event={MEN_6_TO_8} />, { config: BASE_CONFIG });

    expect(screen.getAllByRole('radio', { name: 'Caballero' })).toHaveLength(1);
    expect(screen.getAllByLabelText(/categoría/i)).toHaveLength(1);
  });

  it('sends the declaration with the sign-up', async () => {
    const bodies = captureSignUp();
    const user = userEvent.setup();
    renderBooking(<EventSignUpForm slug="test-club" event={MEN_6_TO_8} />, { config: BASE_CONFIG });

    await user.type(screen.getByLabelText(/nombre/i), 'Ana');
    await user.type(screen.getByLabelText(/apellido/i), 'Diaz');
    await user.type(screen.getByLabelText(/tel.fono/i), '1123456789');
    await user.click(screen.getByRole('radio', { name: 'Caballero' }));
    await user.selectOptions(screen.getByLabelText(/categoría/i), '7');
    await user.click(screen.getByRole('button', { name: /pagar/i }));

    await waitFor(() => {
      expect(bodies).toHaveLength(1);
    });
    expect(bodies[0]).toMatchObject({ gender: 'men', level: 7 });
  });

});

describe('EventSignUpForm declarations: refusals', () => {
  beforeEach(resetTurnstile);

  it('refuses a gender the event does not take before anything is sent', async () => {
    const bodies = captureSignUp();
    const user = userEvent.setup();
    renderBooking(<EventSignUpForm slug="test-club" event={MEN_6_TO_8} />, { config: BASE_CONFIG });

    await user.type(screen.getByLabelText(/nombre/i), 'Ana');
    await user.type(screen.getByLabelText(/apellido/i), 'Diaz');
    await user.type(screen.getByLabelText(/tel.fono/i), '1123456789');
    await user.click(screen.getByRole('radio', { name: 'Dama' }));
    await user.selectOptions(screen.getByLabelText(/categoría/i), '7');
    await user.click(screen.getByRole('button', { name: /pagar/i }));

    expect(await screen.findByText(BOOKING_MESSAGES.declaration.openPlayOnly('Caballeros'))).toBeInTheDocument();
    expect(bodies).toHaveLength(0);
  });

  it('refuses a level outside the range before anything is sent', async () => {
    const bodies = captureSignUp();
    const user = userEvent.setup();
    renderBooking(<EventSignUpForm slug="test-club" event={MEN_6_TO_8} />, { config: BASE_CONFIG });

    await user.type(screen.getByLabelText(/nombre/i), 'Ana');
    await user.type(screen.getByLabelText(/apellido/i), 'Diaz');
    await user.type(screen.getByLabelText(/tel.fono/i), '1123456789');
    await user.click(screen.getByRole('radio', { name: 'Caballero' }));
    await user.selectOptions(screen.getByLabelText(/categoría/i), '5');
    await user.click(screen.getByRole('button', { name: /pagar/i }));

    expect(await screen.findByText('Esta cancha abierta es de 6ta a 8va.')).toBeInTheDocument();
    expect(bodies).toHaveLength(0);
  });

  it('shows the words of a refusal the server gives with a code', async () => {
    captureSignUp(422, {
      type: 'https://vibe.com.ar/problems/validation',
      title: 'Validation failed',
      status: 422,
      errors: [{ field: 'level', message: 'level_out_of_range' }],
    });
    const user = userEvent.setup();
    renderBooking(<EventSignUpForm slug="test-club" event={MEN_6_TO_8} />, { config: BASE_CONFIG });

    await user.type(screen.getByLabelText(/nombre/i), 'Ana');
    await user.type(screen.getByLabelText(/apellido/i), 'Diaz');
    await user.type(screen.getByLabelText(/tel.fono/i), '1123456789');
    await user.click(screen.getByRole('radio', { name: 'Caballero' }));
    await user.selectOptions(screen.getByLabelText(/categoría/i), '8');
    await user.click(screen.getByRole('button', { name: /pagar/i }));

    expect(await screen.findByText('Esta cancha abierta es de 6ta a 8va.')).toBeInTheDocument();
  });
});

describe('EventSignUpForm by sport', () => {
  beforeEach(resetTurnstile);

  it('asks a fútbol event with a modality for the gender alone, in the words of the sport', () => {
    const soccer: PublicOpenPlayEvent = { ...EVENT, id: 'ev2', sport: 'soccer', modality: 'women', level_min: null, level_max: null };
    renderBooking(<EventSignUpForm slug="test-club" event={soccer} />, { config: BASE_CONFIG });

    expect(screen.getByRole('heading', { name: 'Femenino' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'Femenino' })).toBeInTheDocument();
    expect(screen.queryByLabelText(/categoría/i)).not.toBeInTheDocument();
  });
});
