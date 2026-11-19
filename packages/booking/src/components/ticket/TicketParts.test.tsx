import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { TicketStub } from './TicketParts';

describe('TicketStub', () => {
  it('carves two notches out of the card edges by default', () => {
    const { container } = render(<TicketStub />);

    expect(container.querySelectorAll('[aria-hidden="true"]')).toHaveLength(2);
  });

  it('renders bare inside a band with no notches', () => {
    const { container } = render(<TicketStub bare />);

    expect(container.querySelectorAll('[aria-hidden="true"]')).toHaveLength(0);
  });
});
