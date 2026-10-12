import type { ComponentProps } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';
import { createBookingWrapper, renderBooking } from '../../test/booking';
import { BOOKING_MESSAGES } from '../../messages';
import type { PublicComplex } from '../../api/types';
import { ComplexPageContent } from './ComplexPageContent';

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

/** The server HTML, parsed back into a tree so document order can be compared. */
function serverDom(): HTMLDivElement {
  const Wrapper = createBookingWrapper();
  const container = document.createElement('div');
  container.innerHTML = renderToStaticMarkup(
    <Wrapper>
      <ComplexPageContent {...baseProps} />
    </Wrapper>,
  );
  return container;
}

describe('ComplexPageContent in the server markup', () => {
  // Crawlers get this markup. The services sit once in the header, and the
  // header comes before the date strip, which is where they sit at `lg`.
  it('writes the services once, before the date picker', () => {
    const container = serverDom();
    const headings = Array.from(container.querySelectorAll('h2'));
    const services = headings.find((h) => h.textContent === BOOKING_MESSAGES.complex.amenitiesSection);
    const dates = Array.from(container.querySelectorAll('[role="radiogroup"]')).find(
      (g) => g.getAttribute('aria-label') === BOOKING_MESSAGES.publicBooking.dateStripLabel,
    );

    expect(headings.filter((h) => h.textContent === BOOKING_MESSAGES.complex.amenitiesSection)).toHaveLength(1);
    expect(
      services && dates && services.compareDocumentPosition(dates) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
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
  it('places the services before the date picker, and only once, in the browser too', () => {
    const spy = vi.spyOn(window, 'matchMedia');

    renderBooking(<ComplexPageContent {...baseProps} />);

    const services = screen.getByRole('heading', { level: 2, name: BOOKING_MESSAGES.complex.amenitiesSection });
    const dates = screen.getByRole('radiogroup', { name: BOOKING_MESSAGES.publicBooking.dateStripLabel });
    expect(services.compareDocumentPosition(dates) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(spy).not.toHaveBeenCalled();
  });
});
