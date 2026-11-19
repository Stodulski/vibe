import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { CancelledResult } from '../../components/registration/CancelRegistration';

describe('cancelled states', () => {
  it('shows the refund once, in the server sentence, not again as an amount', () => {
    const { container } = render(
      <CancelledResult
        refund={{ status: 'issued', message: 'Te devolvimos $11.000.', amount: 1_100_000 }}
        slug="test-club"
        title="Lugar cancelado"
      />,
    );
    expect(screen.getByText('Te devolvimos $11.000.')).toBeInTheDocument();
    expect(container.textContent.match(/\$11\.000/g)).toHaveLength(1);
  });
});
