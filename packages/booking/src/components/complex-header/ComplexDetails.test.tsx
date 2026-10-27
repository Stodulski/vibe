import { render, screen } from '@testing-library/react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

import { BOOKING_MESSAGES } from '../../messages';
import type { Schedule } from '../../api/types';
import { ComplexDetails } from './ComplexDetails';

const t = BOOKING_MESSAGES;

const schedules: Schedule[] = [
  {
    id: 's1',
    complex_id: 'c1',
    day: 'monday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
];

describe('ComplexDetails in the server markup', () => {
  // Crawlers and no-JS readers get only the server HTML. The week must be in it
  // at every width, not only inside an open panel.
  it('writes the weekly hours into the HTML, not only inside a collapsed panel', () => {
    const html = renderToStaticMarkup(<ComplexDetails schedules={schedules} />);

    expect(html).toContain('08:00 - 23:00');
  });

  it('never asks matchMedia where the hours go', () => {
    const spy = vi.spyOn(window, 'matchMedia');

    renderToStaticMarkup(<ComplexDetails schedules={schedules} />);

    expect(spy).not.toHaveBeenCalled();
  });
});

describe('ComplexDetails in the browser', () => {
  // On a phone too: the hours are open, with no tap to reveal them.
  it('shows the week with no trigger to open it', () => {
    render(<ComplexDetails schedules={schedules} />);

    expect(screen.getByText('08:00 - 23:00')).toBeVisible();
    expect(screen.queryByRole('button', { name: t.complex.schedules })).not.toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 2, name: t.complex.schedules })).toBeInTheDocument();
  });
});
