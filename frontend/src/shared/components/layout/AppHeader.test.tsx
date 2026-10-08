import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { AppHeader } from './AppHeader';

describe('AppHeader', () => {
  it('renders the logo image with correct alt text', () => {
    render(
      <MemoryRouter>
        <AppHeader />
      </MemoryRouter>,
    );
    const logo = screen.getByAltText('Vibe');
    expect(logo).toBeInTheDocument();
    expect(logo).toHaveAttribute('src', '/logo.svg');
  });

  it('renders children when provided', () => {
    render(
      <MemoryRouter>
        <AppHeader>
          <span data-testid="child">Child Content</span>
        </AppHeader>
      </MemoryRouter>,
    );
    expect(screen.getByTestId('child')).toBeInTheDocument();
  });

  it('links to home by default when no slug', () => {
    render(
      <MemoryRouter>
        <AppHeader />
      </MemoryRouter>,
    );
    const link = screen.getByAltText('Vibe').closest('a');
    expect(link).toHaveAttribute('href', '/');
  });

  it('links to the complex storefront under /c/ when the route carries a slug', () => {
    render(
      <MemoryRouter initialEntries={['/c/los-alamos/book']}>
        <Routes>
          <Route path="/c/:slug/*" element={<AppHeader />} />
        </Routes>
      </MemoryRouter>,
    );
    const link = screen.getByAltText('Vibe').closest('a');
    expect(link).toHaveAttribute('href', '/c/los-alamos');
  });
});
