import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StatTile } from './StatTile';

describe('StatTile', () => {
  it('renders the label and value', () => {
    render(<StatTile label="Reservas" value="42" />);
    expect(screen.getByText('Reservas')).toBeInTheDocument();
    expect(screen.getByText('42')).toBeInTheDocument();
  });

  it('clamps aria-valuenow to the same range as the rendered width (B11)', () => {
    render(<StatTile label="Ocupación" value="120%" progressBar={120} />);
    const bar = screen.getByRole('progressbar');
    expect(bar).toHaveAttribute('aria-valuenow', '100');
  });

  it('clamps a negative progressBar to 0', () => {
    render(<StatTile label="Ocupación" value="-10%" progressBar={-10} />);
    const bar = screen.getByRole('progressbar');
    expect(bar).toHaveAttribute('aria-valuenow', '0');
  });

  it('does not render a progressbar when progressBar is omitted', () => {
    render(<StatTile label="Reservas" value="42" />);
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
  });
});
