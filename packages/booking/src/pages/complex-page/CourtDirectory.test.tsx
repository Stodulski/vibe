import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { CourtDirectory } from './CourtDirectory';

const base = { id: 'c1', name: 'cancha principal', sport: 'soccer', court_type: 'outdoor', has_lighting: false } as const;

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

  it('always says the roof type, and nothing about surface or lighting when there are none', () => {
    render(<CourtDirectory courts={[{ ...base }]} />);
    expect(screen.getByText('Descubierta')).toBeInTheDocument();
    expect(screen.queryByText('Con iluminación')).not.toBeInTheDocument();
  });

  it('says the surface when set and the lighting when the court has it', () => {
    render(<CourtDirectory courts={[{ ...base, surface: 'synthetic_grass', has_lighting: true }]} />);
    expect(screen.getByText('Descubierta')).toBeInTheDocument();
    expect(screen.getByText('Césped sintético')).toBeInTheDocument();
    expect(screen.getByText('Con iluminación')).toBeInTheDocument();
  });
});
