import { render, screen } from '@testing-library/react';
import { renderToStaticMarkup } from 'react-dom/server';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { BOOKING_MESSAGES } from '../../messages';
import { ComplexDetails } from './ComplexDetails';

const noop = () => undefined;

/* Por debajo de lg el encabezado se apila y los dos bloques son acordeones. */
function stackedViewport() {
  vi.spyOn(window, 'matchMedia').mockImplementation(query => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: noop,
    removeEventListener: noop,
    addListener: noop,
    removeListener: noop,
    dispatchEvent: () => false,
  }));
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('ComplexDetails heading outline', () => {
  it('opens the schedules accordion header as h2, right under the page h1', () => {
    stackedViewport();
    render(<ComplexDetails schedules={[]} amenities={[]} />);

    expect(screen.getByRole('heading', { level: 2, name: BOOKING_MESSAGES.complex.schedules })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { level: 3 })).not.toBeInTheDocument();
  });
});

describe('ComplexDetails in the server markup', () => {
  // Crawlers and no-JS readers get the server HTML only. A closed accordion
  // panel is not rendered there at all, so the services must sit outside it.
  it('writes every service into the HTML, not only inside a collapsed panel', () => {
    const html = renderToStaticMarkup(<ComplexDetails schedules={[]} amenities={['parking', 'wifi']} />);

    expect(html).toContain(BOOKING_MESSAGES.complex.amenities.parking);
    expect(html).toContain(BOOKING_MESSAGES.complex.amenities.wifi);
  });
});
