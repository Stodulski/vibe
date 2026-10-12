import type { ComponentProps } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';
import { createBookingWrapper, renderBooking } from '../../test/booking';
import { BOOKING_MESSAGES } from '../../messages';
import type { PublicComplex } from '../../api/types';
import { ComplexPageContent } from './ComplexPageContent';

// Leaflet is client-only and not under test here; the map block is covered by ComplexLower.
vi.mock('../../components/ComplexMap', () => ({
  ComplexMap: () => <div data-testid="mock-map">Map</div>,
}));

const complex: PublicComplex = {
  id: 'c1',
  name: 'Club Padel Norte',
  slug: 'club-padel-norte',
  amenities: ['parking', 'wifi'],
  address: 'Av. Libertador 1234',
  city: 'Buenos Aires',
  province: 'CABA',
  country_code: 'AR',
  currency: 'ARS',
  phone: '+5491155550000',
  deposit_percentage: 30,
  cancellation_hours: 24,
  payments_enabled: true,
};

const baseProps: ComponentProps<typeof ComplexPageContent> = {
  complex,
  schedules: [],
  courts: [],
  mpConnected: true,
  selectedDate: new Date(2026, 2, 20),
  onDateSelect: vi.fn(),
  availableSports: [],
  sportFilter: null,
  onSportFilterChange: vi.fn(),
  availLoading: false,
  availError: false,
  onAvailRetry: vi.fn(),
  availStale: false,
  availability: undefined,
  dateStr: '2026-03-20',
  filteredCourts: [],
  selectedSlot: null,
  onSelectSlot: vi.fn(),
  onContinue: vi.fn(),
  duration: 90,
  selectedDuration: null,
  onDurationChange: vi.fn(),
  pendingStartTime: null,
  onPendingStartTimeChange: vi.fn(),
  answeredFromUrl: { sport: false, duration: false },
  emptyDay: null,
};

afterEach(() => {
  vi.restoreAllMocks();
});

const withMap: PublicComplex = { ...complex, latitude: -34.6, longitude: -58.4 };

/** True when `later` comes after `earlier` in document order. */
function follows(earlier: Element | undefined, later: Element | undefined): boolean {
  return Boolean(earlier && later && earlier.compareDocumentPosition(later) & Node.DOCUMENT_POSITION_FOLLOWING);
}

/** The server HTML, parsed back into a tree so document order can be compared. */
function serverDom(props: Partial<ComponentProps<typeof ComplexPageContent>> = {}): HTMLDivElement {
  const Wrapper = createBookingWrapper();
  const container = document.createElement('div');
  container.innerHTML = renderToStaticMarkup(
    <Wrapper>
      <ComplexPageContent {...baseProps} {...props} />
    </Wrapper>,
  );
  return container;
}

describe('ComplexPageContent in the server markup', () => {
  // The DOM order is the reading and focus order at every width, so the date
  // strip comes first, where a phone shows it. Services and map follow it.
  it('writes the date strip before the services and the map, and the services once', () => {
    const container = serverDom({ complex: withMap });
    const headings = Array.from(container.querySelectorAll('h2'));
    const services = headings.find((h) => h.textContent === BOOKING_MESSAGES.complex.amenitiesSection);
    const dates = Array.from(container.querySelectorAll('[role="radiogroup"]')).find(
      (g) => g.getAttribute('aria-label') === BOOKING_MESSAGES.publicBooking.dateStripLabel,
    );
    const map = Array.from(container.querySelectorAll('a')).find(
      (a) => a.textContent === BOOKING_MESSAGES.complex.viewOnGoogleMaps,
    );

    expect(headings.filter((h) => h.textContent === BOOKING_MESSAGES.complex.amenitiesSection)).toHaveLength(1);
    expect(follows(dates, services)).toBe(true);
    expect(follows(dates, map)).toBe(true);
  });

  it('lays the page out as one flex column', () => {
    const container = serverDom();
    const root = container.firstElementChild;

    expect(root?.classList.contains('flex')).toBe(true);
    expect(root?.classList.contains('flex-col')).toBe(true);
  });

  it('never asks matchMedia where the services go', () => {
    const spy = vi.spyOn(window, 'matchMedia');

    serverDom();

    expect(spy).not.toHaveBeenCalled();
  });
});

describe('ComplexPageContent placement of the services', () => {
  it('places the date strip before the services and the map in the browser too', () => {
    const spy = vi.spyOn(window, 'matchMedia');

    renderBooking(<ComplexPageContent {...baseProps} complex={withMap} />);

    const services = screen.getByRole('heading', { level: 2, name: BOOKING_MESSAGES.complex.amenitiesSection });
    const dates = screen.getByRole('radiogroup', { name: BOOKING_MESSAGES.publicBooking.dateStripLabel });
    const map = screen.getByRole('link', { name: BOOKING_MESSAGES.complex.viewOnGoogleMaps });
    expect(follows(dates, services)).toBe(true);
    expect(follows(dates, map)).toBe(true);
    expect(spy).not.toHaveBeenCalled();
  });

  it('keeps the services after the phone panel when the club is not booked online', () => {
    const container = serverDom({ complex: withMap, mpConnected: false });
    const headings = Array.from(container.querySelectorAll('h2'));
    const phone = headings.find((h) => h.textContent === BOOKING_MESSAGES.publicBooking.phoneOnlyTitle);
    const services = headings.find((h) => h.textContent === BOOKING_MESSAGES.complex.amenitiesSection);

    expect(phone).toBeDefined();
    expect(follows(phone, services)).toBe(true);
  });
});
