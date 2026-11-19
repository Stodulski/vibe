import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { LoadingButton } from './LoadingButton';

describe('LoadingButton', () => {
  it('keeps the label in the DOM while loading, so the button width is preserved', () => {
    render(<LoadingButton loading>Guardar</LoadingButton>);
    // Still findable by its accessible name — not removed, just visually hidden.
    expect(screen.getByRole('button', { name: 'Guardar' })).toBeInTheDocument();
  });

  it('is disabled and aria-busy while loading', () => {
    render(<LoadingButton loading>Guardar</LoadingButton>);
    const button = screen.getByRole('button', { name: 'Guardar' });
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute('aria-busy', 'true');
  });

  it('is not disabled or busy when not loading', () => {
    render(<LoadingButton>Guardar</LoadingButton>);
    const button = screen.getByRole('button', { name: 'Guardar' });
    expect(button).not.toBeDisabled();
    expect(button).toHaveAttribute('aria-busy', 'false');
  });

  it('stays disabled when explicitly disabled even without loading', () => {
    render(<LoadingButton disabled>Guardar</LoadingButton>);
    expect(screen.getByRole('button', { name: 'Guardar' })).toBeDisabled();
  });
});
