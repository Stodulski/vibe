import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { toast } from 'sonner';
import { server } from '../../test/msw/server';
import { renderBooking } from '../../test/booking';
import { CAT_OPEN, TOURNAMENT } from '../../test/tournamentFixtures';
import type { BookingConfig } from '../../config';
import type { PublicTournamentCategory } from '../../api/types';
import { BOOKING_MESSAGES } from '../../messages';
import { PairSignUpForm } from './PairSignUpForm';

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
    http.post('*/public/complexes/test-club/tournaments/t1/categories/c-open/pairs', async ({ request }) => {
      bodies.push((await request.json()) as Record<string, unknown>);
      if (status !== 201) return HttpResponse.json(body, { status });
      return HttpResponse.json({ token: 'tok', mp_init_point: 'https://mp.test/pay' }, { status });
    }),
  );
  return bodies;
}

/** Types into the `index`-th field with this label: 0 is the payer, 1 the partner. */
async function typeInto(user: ReturnType<typeof userEvent.setup>, label: RegExp, index: number, text: string) {
  const field = screen.getAllByLabelText(label)[index];
  if (!field) throw new Error(`no field ${String(label)} #${String(index)}`);
  await user.type(field, text);
}

async function fillAndPay(user: ReturnType<typeof userEvent.setup>) {
  await typeInto(user, /nombre/i, 0, 'Ana');
  await typeInto(user, /apellido/i, 0, 'Diaz');
  await typeInto(user, /tel.fono/i, 0, '1123456789');
  await typeInto(user, /nombre/i, 1, 'Beto');
  await typeInto(user, /apellido/i, 1, 'Socio');
  await typeInto(user, /tel.fono/i, 1, '1198765432');
  await user.click(screen.getByRole('button', { name: /pagar/i }));
}

function resetTurnstile() {
  turnstile.onSuccess = undefined;
  turnstile.mounts.count = 0;
  vi.mocked(toast.info).mockClear();
}

describe('PairSignUpForm Turnstile', () => {
  beforeEach(resetTurnstile);

  it('sends the solved token with the sign-up, like the booking hold', async () => {
    const bodies = captureSignUp();
    const user = userEvent.setup();
    renderBooking(<PairSignUpForm slug="test-club" tournament={TOURNAMENT} category={CAT_OPEN} />, { config: TURNSTILE_CONFIG });
    act(() => {
      turnstile.onSuccess?.('solved-token');
    });

    await fillAndPay(user);

    await waitFor(() => {
      expect(bodies).toHaveLength(1);
    });
    expect(bodies[0]).toMatchObject({ first_name: 'Ana', partner: { first_name: 'Beto' }, turnstile_token: 'solved-token' });
  });

  it('waits for the challenge when it is not solved yet, and signs up once it is', async () => {
    const bodies = captureSignUp();
    const user = userEvent.setup();
    renderBooking(<PairSignUpForm slug="test-club" tournament={TOURNAMENT} category={CAT_OPEN} />, { config: TURNSTILE_CONFIG });

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

describe('PairSignUpForm Turnstile refusal', () => {
  beforeEach(resetTurnstile);

  it('shows the failure message and asks for a fresh token when the server refuses it', async () => {
    captureSignUp(422, {
      type: 'https://vibe.com.ar/problems/validation',
      title: 'Validation failed',
      status: 422,
      errors: [{ field: 'turnstile_token', message: 'invalid' }],
    });
    const user = userEvent.setup();
    renderBooking(<PairSignUpForm slug="test-club" tournament={TOURNAMENT} category={CAT_OPEN} />, { config: TURNSTILE_CONFIG });
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
    renderBooking(<PairSignUpForm slug="test-club" tournament={TOURNAMENT} category={CAT_OPEN} />, { config: BASE_CONFIG });

    await fillAndPay(user);

    await waitFor(() => {
      expect(bodies).toHaveLength(1);
    });
    expect(bodies[0]).not.toHaveProperty('turnstile_token');
    expect(screen.queryByTestId('turnstile-widget')).not.toBeInTheDocument();
  });
});

/** A category of Caballeros 8va: each player must be a man of 8va or weaker. */
const MEN_8VA: PublicTournamentCategory = { ...CAT_OPEN, modality: 'men', level: 8, suma: null };

/** Fills the names and phones of both players, leaving their declarations to the test. */
async function fillPlayers(user: ReturnType<typeof userEvent.setup>) {
  await typeInto(user, /nombre/i, 0, 'Ana');
  await typeInto(user, /apellido/i, 0, 'Diaz');
  await typeInto(user, /tel.fono/i, 0, '1123456789');
  await typeInto(user, /nombre/i, 1, 'Beto');
  await typeInto(user, /apellido/i, 1, 'Socio');
  await typeInto(user, /tel.fono/i, 1, '1198765432');
}

/** Declares one player: 0 is the payer, 1 the partner. The gender is the segment's name, the category its level. */
async function declarePlayer(user: ReturnType<typeof userEvent.setup>, index: number, gender: 'Caballero' | 'Dama', level: string) {
  const choice = screen.getAllByRole('radio', { name: gender })[index];
  const category = screen.getAllByLabelText(/categoría/i)[index];
  if (!choice || !category) throw new Error(`no declaration #${String(index)}`);
  await user.click(choice);
  await user.selectOptions(category, level);
}

describe('PairSignUpForm declarations', () => {
  beforeEach(resetTurnstile);

  it('states the requirement of the category under the heading', () => {
    renderBooking(<PairSignUpForm slug="test-club" tournament={TOURNAMENT} category={MEN_8VA} />, { config: BASE_CONFIG });

    expect(screen.getByText('Caballeros · 8va o menor')).toBeInTheDocument();
  });

  it('asks each player for a caballero or a dama and a category, only for a category with a modality', () => {
    renderBooking(<PairSignUpForm slug="test-club" tournament={TOURNAMENT} category={MEN_8VA} />, { config: BASE_CONFIG });

    expect(screen.getAllByRole('radio', { name: 'Caballero' })).toHaveLength(2);
    expect(screen.getAllByRole('radio', { name: 'Dama' })).toHaveLength(2);
    expect(screen.getAllByLabelText(/categoría/i)).toHaveLength(2);
  });

  it('sends each declaration with the sign-up', async () => {
    const bodies = captureSignUp();
    const user = userEvent.setup();
    renderBooking(<PairSignUpForm slug="test-club" tournament={TOURNAMENT} category={MEN_8VA} />, { config: BASE_CONFIG });

    await fillPlayers(user);
    await declarePlayer(user, 0, 'Caballero', '8');
    await declarePlayer(user, 1, 'Caballero', '9');
    await user.click(screen.getByRole('button', { name: /pagar/i }));

    await waitFor(() => {
      expect(bodies).toHaveLength(1);
    });
    expect(bodies[0]).toMatchObject({ gender: 'men', level: 8, partner: { gender: 'men', level: 9 } });
  });

});

describe('PairSignUpForm declarations: refusals', () => {
  beforeEach(resetTurnstile);

  it('refuses a partner of the other gender before anything is sent', async () => {
    const bodies = captureSignUp();
    const user = userEvent.setup();
    renderBooking(<PairSignUpForm slug="test-club" tournament={TOURNAMENT} category={MEN_8VA} />, { config: BASE_CONFIG });

    await fillPlayers(user);
    await declarePlayer(user, 0, 'Caballero', '8');
    await declarePlayer(user, 1, 'Dama', '8');
    await user.click(screen.getByRole('button', { name: /pagar/i }));

    expect(await screen.findByText(BOOKING_MESSAGES.declaration.pairMenOnly)).toBeInTheDocument();
    expect(bodies).toHaveLength(0);
  });

  it('refuses a player stronger than the category, the way a 6ta cannot enter an 8va', async () => {
    const bodies = captureSignUp();
    const user = userEvent.setup();
    renderBooking(<PairSignUpForm slug="test-club" tournament={TOURNAMENT} category={MEN_8VA} />, { config: BASE_CONFIG });

    await fillPlayers(user);
    await declarePlayer(user, 0, 'Caballero', '6');
    await declarePlayer(user, 1, 'Caballero', '8');
    await user.click(screen.getByRole('button', { name: /pagar/i }));

    expect(
      await screen.findByText('Esta categoría es 8va: no pueden anotarse jugadores de una categoría más alta.'),
    ).toBeInTheDocument();
    expect(bodies).toHaveLength(0);
  });

  it('asks for the declaration before it sends', async () => {
    const bodies = captureSignUp();
    const user = userEvent.setup();
    renderBooking(<PairSignUpForm slug="test-club" tournament={TOURNAMENT} category={MEN_8VA} />, { config: BASE_CONFIG });

    await fillPlayers(user);
    await user.click(screen.getByRole('button', { name: /pagar/i }));

    // One message for each player who has not chosen a gender yet.
    expect(await screen.findAllByText(BOOKING_MESSAGES.declaration.genderRequired)).toHaveLength(2);
    expect(bodies).toHaveLength(0);
  });

  it('shows the same words for a refusal the server gives with a code', async () => {
    captureSignUp(422, {
      type: 'https://vibe.com.ar/problems/validation',
      title: 'Validation failed',
      status: 422,
      errors: [{ field: 'partner.level', message: 'level_too_strong' }],
    });
    const user = userEvent.setup();
    renderBooking(<PairSignUpForm slug="test-club" tournament={TOURNAMENT} category={MEN_8VA} />, { config: BASE_CONFIG });

    await fillPlayers(user);
    await declarePlayer(user, 0, 'Caballero', '8');
    await declarePlayer(user, 1, 'Caballero', '8');
    await user.click(screen.getByRole('button', { name: /pagar/i }));

    expect(
      await screen.findByText('Esta categoría es 8va: no pueden anotarse jugadores de una categoría más alta.'),
    ).toBeInTheDocument();
  });

});

describe('PairSignUpForm without a modality', () => {
  beforeEach(resetTurnstile);

  it('keeps a category without a modality as it was: nothing asked, nothing sent', async () => {
    const bodies = captureSignUp();
    const user = userEvent.setup();
    renderBooking(<PairSignUpForm slug="test-club" tournament={TOURNAMENT} category={CAT_OPEN} />, { config: BASE_CONFIG });

    expect(screen.queryByRole('radio')).not.toBeInTheDocument();
    await fillPlayers(user);
    await user.click(screen.getByRole('button', { name: /pagar/i }));

    await waitFor(() => {
      expect(bodies).toHaveLength(1);
    });
    expect(bodies[0]).not.toHaveProperty('gender');
    expect(bodies[0]).not.toHaveProperty('level');
  });
});
