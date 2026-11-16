import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TicketCourt, TicketStub } from './TicketParts';

describe('TicketStub', () => {
  it('carves two notches out of the card edges by default', () => {
    const { container } = render(<TicketStub />);

    expect(container.querySelectorAll('[aria-hidden="true"]')).toHaveLength(2);
    expect(container.firstElementChild).toHaveClass('-mx-6');
  });

  // Inside the band the rule spans the band's inner width: no notches, and no
  // negative margins that would push the rule past the band at 320px.
  it('renders bare inside a band: no notches and no negative margins', () => {
    const { container } = render(<TicketStub bare />);

    expect(container.querySelectorAll('[aria-hidden="true"]')).toHaveLength(0);
    expect(container.querySelector('.-mx-6')).toBeNull();
    expect(container.querySelector('.sm\\:-mx-8')).toBeNull();
    expect(container.firstElementChild).toHaveClass('border-dashed');
  });
});

describe('TicketCourt chips', () => {
  it('keeps the tint of the confirmed ticket outside the band', () => {
    render(<TicketCourt name="Cancha 1" tags={['Techada']} />);

    expect(screen.getByText('Techada')).toHaveClass('text-primary-400');
  });

  // The band re-points primary-400 to the ink, which would vanish on the chip's
  // own dark fill, so a bare chip takes the band's brand green instead.
  it('takes the brand green inside the band, where primary-400 is the ink', () => {
    render(<TicketCourt name="Cancha 1" tags={['Techada']} bare />);

    expect(screen.getByText('Techada')).toHaveClass('text-primary-500');
    expect(screen.getByText('Techada')).not.toHaveClass('text-primary-400');
  });
});
