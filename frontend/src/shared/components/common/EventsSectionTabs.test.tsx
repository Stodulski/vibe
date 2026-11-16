import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { EventsSectionTabs } from './EventsSectionTabs';

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <EventsSectionTabs />
      <Routes>
        <Route path="/events/open-play" element={<p>Lista de canchas libres</p>} />
        <Route path="/events/tournaments" element={<p>Lista de torneos</p>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('EventsSectionTabs', () => {
  it('links the two tabs to their routes', () => {
    renderAt('/events/open-play');
    expect(screen.getByRole('link', { name: 'Canchas libres' })).toHaveAttribute('href', '/events/open-play');
    expect(screen.getByRole('link', { name: 'Torneos' })).toHaveAttribute('href', '/events/tournaments');
  });

  it('marks only the tab of the current route', () => {
    renderAt('/events/tournaments');
    expect(screen.getByRole('link', { name: 'Torneos' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Canchas libres' })).not.toHaveAttribute('aria-current');
  });

  it('switches the content when the other tab is pressed', async () => {
    renderAt('/events/open-play');
    expect(screen.getByText('Lista de canchas libres')).toBeInTheDocument();

    await userEvent.setup().click(screen.getByRole('link', { name: 'Torneos' }));

    expect(screen.getByText('Lista de torneos')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Torneos' })).toHaveAttribute('aria-current', 'page');
  });
});
