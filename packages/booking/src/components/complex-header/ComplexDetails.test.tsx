import { render, screen } from '@testing-library/react';
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
    render(<ComplexDetails schedules={[]} />);

    expect(screen.getByRole('heading', { level: 2, name: BOOKING_MESSAGES.complex.schedules })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { level: 3 })).not.toBeInTheDocument();
  });
});

describe('ComplexDetails on a phone', () => {
  // The slot picker follows the club card directly, so the hours start closed
  // and the visitor opens them only when they want to read the week.
  it('starts with the schedules accordion closed', () => {
    stackedViewport();
    render(<ComplexDetails schedules={[]} />);

    expect(screen.getByRole('button', { name: BOOKING_MESSAGES.complex.schedules })).toHaveAttribute(
      'aria-expanded',
      'false',
    );
  });
});
