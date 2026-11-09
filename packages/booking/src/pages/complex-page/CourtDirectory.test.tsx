import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { CourtDirectory } from './CourtDirectory';

const base = { id: 'c1', name: 'cancha principal', sport: 'soccer', court_type: 'outdoor' } as const;

describe('CourtDirectory', () => {
  it('says "Sin descripción" when the court has no description', () => {
    render(<CourtDirectory courts={[{ ...base }]} />);
    expect(screen.getByText('Sin descripción')).toBeInTheDocument();
  });

  it('says "Sin descripción" when the description is only whitespace', () => {
    render(<CourtDirectory courts={[{ ...base, description: '   ' }]} />);
    expect(screen.getByText('Sin descripción')).toBeInTheDocument();
  });

  it('shows the owner description when there is one', () => {
    render(<CourtDirectory courts={[{ ...base, description: 'Césped nuevo' }]} />);
    expect(screen.getByText('Césped nuevo')).toBeInTheDocument();
    expect(screen.queryByText('Sin descripción')).not.toBeInTheDocument();
  });
});
