import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ResultCountAnnouncer } from './ResultCountAnnouncer';

describe('ResultCountAnnouncer', () => {
  it('announces the count politely, without taking focus', () => {
    render(<ResultCountAnnouncer count={12} />);

    const region = screen.getByText('12 resultados');
    expect(region).toHaveAttribute('aria-live', 'polite');
    expect(region).toHaveClass('sr-only');
  });

  it('uses the singular for one row', () => {
    render(<ResultCountAnnouncer count={1} />);
    expect(screen.getByText('1 resultado')).toBeInTheDocument();
  });

  it('keeps the same region across updates, so the new count is announced', () => {
    const { rerender } = render(<ResultCountAnnouncer count={50} />);
    const region = screen.getByText('50 resultados');

    rerender(<ResultCountAnnouncer count={100} />);

    expect(region).toHaveTextContent('100 resultados');
  });

  it('announces an emptied list rather than going silent', () => {
    render(<ResultCountAnnouncer count={0} />);
    expect(screen.getByText('0 resultados')).toBeInTheDocument();
  });

  // The region must outlive the states around it: mounted while the query is
  // still in flight, and the same node once the answer arrives — a region
  // that appears together with its text is not reliably announced.
  it('stays mounted and silent until there is a settled count', () => {
    const { container, rerender } = render(<ResultCountAnnouncer count={null} />);

    const region = container.querySelector('[aria-live="polite"]');
    expect(region).toBeInTheDocument();
    expect(region).toHaveTextContent('');

    rerender(<ResultCountAnnouncer count={0} />);

    expect(container.querySelector('[aria-live="polite"]')).toBe(region);
    expect(region).toHaveTextContent('0 resultados');
  });
});
