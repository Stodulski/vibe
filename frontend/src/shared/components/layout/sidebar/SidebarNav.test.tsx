import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { SidebarNav } from './SidebarNav';

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <SidebarNav isMobile={false} onPrefetch={() => () => undefined} />
    </MemoryRouter>,
  );
}

describe('SidebarNav — Eventos', () => {
  it('shows one Eventos entry after Reservas instead of Canchas abiertas and Torneos', () => {
    renderAt('/dashboard');
    const names = screen.getAllByRole('link').map((link) => link.textContent);
    expect(names.filter((name) => name === 'Eventos')).toHaveLength(1);
    expect(names.indexOf('Eventos')).toBe(names.indexOf('Reservas') + 1);
    expect(names).not.toContain('Canchas abiertas');
    expect(names).not.toContain('Torneos');
    expect(screen.getByRole('link', { name: 'Eventos' })).toHaveAttribute('href', '/events');
  });

  it.each(['/events', '/events/tournaments/t1'])('marks Eventos active on %s', (path) => {
    renderAt(path);
    expect(screen.getByRole('link', { name: 'Eventos' })).toHaveAttribute('aria-current', 'page');
  });

  it('does not mark Eventos active on /eventsfoo, which only shares the text prefix', () => {
    renderAt('/eventsfoo');
    expect(screen.getByRole('link', { name: 'Eventos' })).not.toHaveAttribute('aria-current', 'page');
  });
});

describe('SidebarNav — Kit para redes', () => {
  it('shows Kit para redes right before Configuración in the secondary section', () => {
    renderAt('/dashboard');
    const names = screen.getAllByRole('link').map((link) => link.textContent);
    expect(names.indexOf('Kit para redes')).toBe(names.indexOf('Configuración') - 1);
    expect(screen.getByRole('link', { name: 'Kit para redes' })).toHaveAttribute('href', '/kit');
  });

  it('marks Kit para redes active on /kit only', () => {
    renderAt('/kit');
    expect(screen.getByRole('link', { name: 'Kit para redes' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Configuración' })).not.toHaveAttribute('aria-current', 'page');
  });
});

describe('SidebarNav — active path resolution', () => {
  it('marks Caja active on the exact /cash route', () => {
    renderAt('/cash');
    expect(screen.getByRole('link', { name: 'Caja' })).toHaveAttribute('aria-current', 'page');
  });

  it('marks Caja active on /cash/sell (T5a: no new sidebar item for the Vender tab)', () => {
    renderAt('/cash/sell');
    expect(screen.getByRole('link', { name: 'Caja' })).toHaveAttribute('aria-current', 'page');
  });

  it('marks Caja active on /cash/products (T5a: no new sidebar item for the Productos tab)', () => {
    renderAt('/cash/products');
    expect(screen.getByRole('link', { name: 'Caja' })).toHaveAttribute('aria-current', 'page');
  });

  it('marks Caja active on a product detail route (/cash/products/:id)', () => {
    renderAt('/cash/products/p1');
    expect(screen.getByRole('link', { name: 'Caja' })).toHaveAttribute('aria-current', 'page');
  });

  it('does not mark Caja active on an unrelated route', () => {
    renderAt('/dashboard');
    expect(screen.getByRole('link', { name: 'Caja' })).not.toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Dashboard' })).toHaveAttribute('aria-current', 'page');
  });

  // A plain `startsWith('/cash')` would wrongly match this sibling route,
  // which only shares the text prefix, not the `/cash` section.
  it('does not mark Caja active on /cashflow, which only shares the text prefix', () => {
    renderAt('/cashflow');
    expect(screen.getByRole('link', { name: 'Caja' })).not.toHaveAttribute('aria-current', 'page');
  });
});
