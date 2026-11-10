import { describe, it, expect, vi, beforeEach, afterEach, type MockInstance } from 'vitest';
import { act, screen, fireEvent, within, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { toast } from 'sonner';
import { server } from '../test/msw/server';
import { renderBooking, setBookingUrl } from '../test/booking';
import { confirmDraftKey, readConfirmDraft, saveHoldExpiredNotice } from '../lib/handoff';
import type { AvailabilityData, PublicComplexResponse } from '../api/types';
import type { BookingConfig } from '../config';
import { BOOKING_MESSAGES } from '../messages';

const t = BOOKING_MESSAGES;

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() } }));

interface CapturedTurnstileProps {
  siteKey?: string;
  options?: Record<string, unknown>;
  onSuccess?: (token: string) => void;
}

const turnstile = vi.hoisted(() => ({
  props: null as CapturedTurnstileProps | null,
  mounts: { count: 0 },
}));

// A stand-in widget: it records its props, so a test can solve the challenge
// with `onSuccess`, and counts its mounts: a fresh token means a fresh widget.
vi.mock('@marsidev/react-turnstile', async () => {
  const { useEffect } = await import('react');
  return {
    Turnstile: (props: CapturedTurnstileProps) => {
      turnstile.props = props;
      useEffect(() => {
        turnstile.mounts.count += 1;
      }, []);
      return <div data-testid="turnstile-widget" />;
    },
  };
});

const mockUseComplexBySlug =
  vi.fn<() => { data: PublicComplexResponse | undefined; isLoading: boolean; error: unknown }>();
vi.mock('../hooks/useComplexBySlug', () => ({
  useComplexBySlug: () => mockUseComplexBySlug(),
}));

const mockUseAvailability = vi.fn<() => { data: AvailabilityData | undefined; isLoading: boolean }>();
vi.mock('../hooks/useAvailability', () => ({
  useAvailability: () => mockUseAvailability(),
}));

vi.mock('../components/ComplexHeader', () => ({
  ComplexHeader: () => <div data-testid="complex-header">Header</div>,
}));
vi.mock('../components/DateSelector', () => ({
  DateSelector: () => <div data-testid="date-selector" />,
}));

const BASE_CONFIG: BookingConfig = {
  apiBaseUrl: 'http://localhost/api/v1',
  publicSiteUrl: 'http://localhost',
  privacyUrl: 'https://vibe.com.ar/privacidad',
};
const TURNSTILE_CONFIG: BookingConfig = { ...BASE_CONFIG, turnstileSiteKey: 'test-site-key' };

const EXPIRES_AT = '2030-01-01T12:15:00.000Z';

function complexData(): PublicComplexResponse {
  return {
    complex: {
      id: 'c1',
      name: 'Club Test',
      slug: 'test-club',
      amenities: [],
      address: 'Calle Falsa 123',
      city: 'CABA',
      province: 'CABA',
      country_code: 'AR',
      currency: 'ARS',
      phone: '+541100000000',
      deposit_percentage: 30,
      cancellation_hours: 24,
      payments_enabled: true,
    },
    courts: [],
    schedules: [],
  };
}

function availability(): AvailabilityData {
  return {
    date: '2026-08-24',
    day: 'monday',
    is_open: true,
    courts: [
      {
        court_id: 'ct1',
        court_name: 'Cancha 1',
        sport: 'padel',
        court_type: 'indoor',
        slots: [
          { start_time: '10:00', end_time: '11:00', start_min: 600, duration_minutes: 60, price: 5000, available: true },
        ],
      },
    ],
  };
}

/** Records every hold request body and answers with `respond`. */
function captureHolds(respond: () => Response = () => HttpResponse.json({ hold: { token: 'hold-new', expires_at: EXPIRES_AT } }, { status: 201 })) {
  const bodies: Record<string, unknown>[] = [];
  server.use(
    http.post('*/book/holds', async ({ request }) => {
      bodies.push((await request.json()) as Record<string, unknown>);
      return respond();
    }),
  );
  return bodies;
}

let assignSpy: MockInstance<Location['assign']> | undefined;

beforeEach(() => {
  vi.clearAllMocks();
  turnstile.props = null;
  turnstile.mounts.count = 0;
  setBookingUrl('/c/test-club?date=2026-08-24&duration=60');
  assignSpy = vi.spyOn(window.location, 'assign').mockImplementation(() => undefined);
  mockUseComplexBySlug.mockReturnValue({ data: complexData(), isLoading: false, error: null });
  mockUseAvailability.mockReturnValue({ data: availability(), isLoading: false });
});

afterEach(() => {
  assignSpy?.mockRestore();
  assignSpy = undefined;
  setBookingUrl('/');
});

async function renderPage(config: BookingConfig) {
  const Page = (await import('./ComplexPage')).default;
  return renderBooking(<Page slug="test-club" />, { config });
}

function solveChallenge(token = 'solved-token') {
  act(() => {
    turnstile.props?.onSuccess?.(token);
  });
}

async function pickCourt() {
  fireEvent.click(screen.getByRole('button', { name: /^10:00\b/ }));
  const courts = await screen.findByRole('group', { name: 'Elegí la cancha' });
  const [courtCard] = within(courts).getAllByRole('button');
  if (!courtCard) throw new Error('expected a court card');
  fireEvent.click(courtCard);
}

describe('ComplexPage slot hold', () => {
  it('renders the Turnstile widget on the complex page with the owner-app options', async () => {
    await renderPage(TURNSTILE_CONFIG);

    expect(screen.getByTestId('turnstile-widget')).toBeInTheDocument();
    expect(turnstile.props?.siteKey).toBe('test-site-key');
    expect(turnstile.props?.options).toEqual({
      theme: 'dark',
      size: 'flexible',
      language: 'es',
      appearance: 'interaction-only',
    });
  });

  it('takes a hold on court pick with the Turnstile token and the previous hold, then moves to confirm', async () => {
    const bodies = captureHolds();
    // A hold left behind by an earlier pick in this tab.
    window.sessionStorage.setItem(
      confirmDraftKey('test-club'),
      JSON.stringify({ ...oldDraft(), holdToken: 'hold-old', holdExpiresAt: EXPIRES_AT }),
    );
    await renderPage(TURNSTILE_CONFIG);
    solveChallenge('tok-1');

    await pickCourt();

    await waitFor(() => {
      expect(assignSpy).toHaveBeenCalledWith('/c/test-club/book/confirm');
    });
    expect(bodies).toHaveLength(1);
    const draft = readConfirmDraft('test-club');
    expect(bodies[0]).toEqual({
      complex_id: 'c1',
      court_id: 'ct1',
      // The page's day: a past `?date=` falls back to today.
      date: draft?.date,
      start_time: '10:00',
      duration_minutes: 60,
      turnstile_token: 'tok-1',
      replaces: 'hold-old',
    });
    // The draft carries the hold, with the server's expiry.
    expect(draft).toEqual(
      expect.objectContaining({ courtId: 'ct1', holdToken: 'hold-new', holdExpiresAt: EXPIRES_AT }),
    );
    // A token is single use: a fresh widget solves the next one.
    await waitFor(() => {
      expect(turnstile.mounts.count).toBe(2);
    });
  });

  it('sends neither turnstile_token nor replaces when the storefront has no site key and no earlier hold', async () => {
    const bodies = captureHolds();
    await renderPage(BASE_CONFIG);
    expect(screen.queryByTestId('turnstile-widget')).not.toBeInTheDocument();

    await pickCourt();

    await waitFor(() => {
      expect(assignSpy).toHaveBeenCalledWith('/c/test-club/book/confirm');
    });
    expect(bodies[0]).not.toHaveProperty('turnstile_token');
    expect(bodies[0]).not.toHaveProperty('replaces');
  });

});

describe('ComplexPage slot hold before the challenge is solved', () => {
  it('waits for the challenge token when the court is picked before it, then takes the hold', async () => {
    const bodies = captureHolds();
    await renderPage(TURNSTILE_CONFIG);

    await pickCourt();

    expect(toast.info).toHaveBeenCalledWith(t.publicBooking.holdChallengePending);
    expect(bodies).toHaveLength(0);
    expect(assignSpy).not.toHaveBeenCalled();

    solveChallenge('late-token');

    await waitFor(() => {
      expect(assignSpy).toHaveBeenCalledWith('/c/test-club/book/confirm');
    });
    expect(bodies).toHaveLength(1);
    expect(bodies[0]).toMatchObject({ court_id: 'ct1', turnstile_token: 'late-token' });
  });
});

describe('ComplexPage slot hold refusals', () => {
  it.each([
    [409, { type: 'https://vibe.com.ar/problems/slot-unavailable', title: 'Conflict', status: 409 }, 'slotConflict'],
    [409, { type: 'https://vibe.com.ar/problems/conflict', title: 'Conflict', status: 409 }, 'bookingConflict'],
    [429, { type: 'https://vibe.com.ar/problems/rate-limited', title: 'Too many', status: 429 }, 'holdRateLimited'],
    [
      422,
      {
        type: 'https://vibe.com.ar/problems/validation',
        title: 'Validation failed',
        status: 422,
        errors: [{ field: 'turnstile_token', message: 'invalid' }],
      },
      'turnstileInvalid',
    ],
    [
      422,
      {
        type: 'https://vibe.com.ar/problems/validation',
        title: 'Validation failed',
        status: 422,
        errors: [{ field: 'turnstile_token', message: 'required' }],
      },
      'turnstileRequired',
    ],
    [500, {}, 'holdCreateError'],
  ] as const)('maps a %i refusal to its Spanish message and stays on the page', async (status, body, key) => {
    captureHolds(() => HttpResponse.json(body, { status }));
    await renderPage(TURNSTILE_CONFIG);
    solveChallenge();

    await pickCourt();

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(t.publicBooking[key]);
    });
    expect(assignSpy).not.toHaveBeenCalled();
    expect(readConfirmDraft('test-club')).toBeNull();
    // The spent token is thrown away even on a refusal.
    await waitFor(() => {
      expect(turnstile.mounts.count).toBe(2);
    });
  });

  it('tells the visitor their hold expired when the confirm page sent them back for that', async () => {
    saveHoldExpiredNotice('test-club');
    await renderPage(BASE_CONFIG);

    expect(await screen.findByRole('alert')).toHaveTextContent(t.publicBooking.holdExpired);
  });

  it('shows the expired notice only once', async () => {
    saveHoldExpiredNotice('test-club');
    const first = await renderPage(BASE_CONFIG);
    await screen.findByRole('alert');
    first.unmount();

    await renderPage(BASE_CONFIG);
    await screen.findByRole('button', { name: /^10:00\b/ });
    expect(screen.queryByText(t.publicBooking.holdExpired)).not.toBeInTheDocument();
  });
});

/** A valid slot draft, as an earlier pick would have left it. */
function oldDraft() {
  return {
    complexId: 'c1',
    complexName: 'Club Test',
    complexPhone: '+541100000000',
    courtId: 'ct1',
    courtName: 'Cancha 1',
    date: '2026-08-24',
    startTime: '09:00',
    endTime: '10:00',
    durationMinutes: 60,
    price: 5000,
    depositPercentage: 30,
    cancellationHours: 24,
  };
}
