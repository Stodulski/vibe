import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { AdminPlaceholderPage } from './AdminPlaceholderPage';

describe('AdminPlaceholderPage', () => {
  it('renders the page title', () => {
    render(<AdminPlaceholderPage />);

    expect(screen.getByRole('heading', { name: 'Panel admin' })).toBeInTheDocument();
  });

  it('renders the empty-state title', () => {
    render(<AdminPlaceholderPage />);

    expect(screen.getByRole('heading', { name: 'En construcción' })).toBeInTheDocument();
  });

  it('renders the placeholder description', () => {
    render(<AdminPlaceholderPage />);

    expect(screen.getByText('Estamos rearmando este panel.')).toBeInTheDocument();
  });
});
