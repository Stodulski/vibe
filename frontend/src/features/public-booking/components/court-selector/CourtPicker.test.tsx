import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CourtPicker } from './CourtPicker';
import { buildTimeOptions } from './timeOptions';
import type { CourtAtTime } from './timeOptions';
import type { AvailabilitySlot, CourtAvailability } from '@/shared/types/api.types';

function slot(start: string, price: number): AvailabilitySlot {
  const [h = '0', m = '0'] = start.split(':');
  return {
    start_time: start,
    end_time: `${start}-end`,
    start_min: Number(h) * 60 + Number(m),
    duration_minutes: 90,
    price,
    available: true,
  };
}

function court(
  id: string,
  type: CourtAvailability['court_type'],
  price: number,
  description?: string,
): CourtAvailability {
  return {
    court_id: id,
    court_name: `Cancha ${id}`,
    sport: 'padel',
    court_type: type,
    ...(description !== undefined ? { description } : {}),
    slots: [slot('20:00', price)],
  };
}

// Two courts differing in type so `needsCourtChoice` is true and the picker
// actually renders its cards.
function mixedOption() {
  const [option] = buildTimeOptions([court('1', 'indoor', 5000, 'Césped sintético'), court('2', 'outdoor', 5000)]);
  if (!option) throw new Error('fixture must produce one time option');
  return option;
}

describe('CourtPicker — a court description', () => {
  it('shows the fallback text, italicised, when a court has no description', () => {
    render(<CourtPicker option={mixedOption()} selectedCourtId={null} onSelect={vi.fn()} />);

    const fallback = screen.getByText('Sin descripción');
    expect(fallback).toBeInTheDocument();
    expect(fallback).toHaveClass('italic');
  });

  it('shows the real description, not the fallback, when the court has one', () => {
    render(<CourtPicker option={mixedOption()} selectedCourtId={null} onSelect={vi.fn()} />);

    expect(screen.getByText('Césped sintético')).toBeInTheDocument();
  });
});

describe('CourtPicker — the court name', () => {
  it('truncates visually and keeps the full name in a title attribute', () => {
    render(<CourtPicker option={mixedOption()} selectedCourtId={null} onSelect={vi.fn()} />);

    const name = screen.getByText('Cancha 1');
    expect(name).toHaveClass('truncate');
    expect(name).toHaveAttribute('title', 'Cancha 1');
  });
});

describe('CourtPicker — selection', () => {
  it('renders the cards in the option courts order', () => {
    render(<CourtPicker option={mixedOption()} selectedCourtId={null} onSelect={vi.fn()} />);

    const cards = screen.getAllByRole('button');
    expect(cards[0]).toHaveTextContent('Cancha 1');
    expect(cards[1]).toHaveTextContent('Cancha 2');
  });

  it('calls onSelect with the clicked court entry', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn<(entry: CourtAtTime) => void>();
    render(<CourtPicker option={mixedOption()} selectedCourtId={null} onSelect={onSelect} />);

    await user.click(screen.getByRole('button', { name: /^Cancha 2\b/ }));

    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onSelect.mock.calls[0]?.[0].court.court_id).toBe('2');
  });

  it('marks only the selected court as pressed', () => {
    render(<CourtPicker option={mixedOption()} selectedCourtId="2" onSelect={vi.fn()} />);

    expect(screen.getByRole('button', { name: /^Cancha 1\b/ })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('button', { name: /^Cancha 2\b/ })).toHaveAttribute('aria-pressed', 'true');
  });
});

describe('CourtPicker — nothing to choose between', () => {
  it('renders nothing when the free courts are identical in type and price', () => {
    const [option] = buildTimeOptions([court('1', 'indoor', 5000), court('2', 'indoor', 5000)]);
    if (!option) throw new Error('fixture must produce one time option');

    const { container } = render(<CourtPicker option={option} selectedCourtId={null} onSelect={vi.fn()} />);

    expect(container.firstChild).toBeNull();
  });
});
