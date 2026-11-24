import { screen } from '@testing-library/react';
import type userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, vi, type Mock, type MockInstance } from 'vitest';
import { renderBooking, setBookingUrl } from './booking';
import { makeConsumedHttpError } from './factories';
import type { BookingConfig } from '../config';
import { confirmDraftKey } from '../lib/handoff';

/**
 * The confirm page and its slot hold under test: the slot, the URLs the hold's
 * beacons and the hours go to, and the browser events that end or restore the
 * page. Each test file mocks `../api/public-booking.api` itself and hands
 * `setUpConfirmHold` its `reclaimHold` mock.
 */

export const SLOT = {
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

const CONFIG: BookingConfig = {
  apiBaseUrl: 'http://localhost/api/v1',
  publicSiteUrl: 'http://localhost',
  privacyUrl: 'https://vibe.com.ar/privacidad',
  turnstileSiteKey: 'test-site-key',
};

export const RELEASE_URL = 'http://localhost/api/v1/book/holds/hold-abc/release';
export const LEAVE_URL = 'http://localhost/api/v1/book/holds/hold-abc/leave';
export const HOURS_URL = '/c/club-norte?date=2026-03-20&duration=90&sport=padel#horarios';

export function inSeconds(seconds: number): string {
  return new Date(Date.now() + seconds * 1000).toISOString();
}

export interface ConfirmHoldEnvironment {
  replaceSpy: MockInstance<Location['replace']>;
  beacon: Mock<(url: string | URL, data?: BodyInit | null) => boolean>;
}

/**
 * Spies on `location.replace` and `navigator.sendBeacon` for each test, and
 * answers every reclaim with a hold still the visitor's unless a test says
 * otherwise.
 */
export function setUpConfirmHold(reclaimHold: Mock<(token: string) => Promise<void>>): ConfirmHoldEnvironment {
  const env = {} as ConfirmHoldEnvironment;
  beforeEach(() => {
    vi.clearAllMocks();
    reclaimHold.mockResolvedValue(undefined);
    env.replaceSpy = vi.spyOn(window.location, 'replace').mockImplementation(() => undefined);
    env.beacon = vi.fn<(url: string | URL, data?: BodyInit | null) => boolean>(() => true);
    Object.defineProperty(navigator, 'sendBeacon', { configurable: true, writable: true, value: env.beacon });
  });
  afterEach(() => {
    env.replaceSpy.mockRestore();
    setBookingUrl('/');
    window.sessionStorage.clear();
  });
  return env;
}

export async function renderPage(draft: unknown) {
  const Page = (await import('../pages/BookConfirmPage')).default;
  setBookingUrl('/c/club-norte/book/confirm');
  window.sessionStorage.setItem(confirmDraftKey('club-norte'), JSON.stringify(draft));
  return renderBooking(<Page slug="club-norte" />, { config: CONFIG });
}

export async function renderHeld(expiresInSeconds = 9 * 60) {
  await renderPage({ ...SLOT, holdToken: 'hold-abc', holdExpiresAt: inSeconds(expiresInSeconds) });
  await screen.findByText('Cancha 1');
}

export async function fillAndPay(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText(/nombre/i), 'Juan');
  await user.type(screen.getByLabelText(/apellido/i), 'Perez');
  await user.type(screen.getByLabelText(/tel.fono/i), '1122334455');
  await user.type(screen.getByLabelText(/email/i), 'juan@example.com');
  await user.click(screen.getByRole('button', { name: /pagar|reservar/i }));
}

export function firePagehide() {
  window.dispatchEvent(new Event('pagehide'));
}

/** The server's answer for a hold that is gone (released after the grace, run out). */
export async function holdGone(): Promise<never> {
  throw await makeConsumedHttpError(409, {
    type: 'https://vibe.com.ar/problems/hold-expired',
    title: 'Hold expired',
    status: 409,
  });
}

/** The browser restoring this document from the back/forward cache. */
export function fireRestore() {
  const event = new Event('pageshow') as PageTransitionEvent;
  Object.defineProperty(event, 'persisted', { value: true });
  window.dispatchEvent(event);
}
