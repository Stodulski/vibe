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
});
