import { render, screen } from '@testing-library/react';
import { SkeletonCard, SkeletonTable, SkeletonStat } from './Skeletons';

describe('Skeleton primitives', () => {
  it('SkeletonCard is aria-hidden', () => {
    const { container } = render(<SkeletonCard />);
    expect(container.firstChild).toHaveAttribute('aria-hidden', 'true');
  });

  it('SkeletonTable renders with default rows', () => {
    render(<SkeletonTable />);
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('SkeletonTable has loading aria-label', () => {
    render(<SkeletonTable />);
    expect(screen.getByRole('status')).toHaveAttribute('aria-label', 'Cargando...');
  });

  it('SkeletonTable with live={false} is aria-hidden and has no status role', () => {
    const { container } = render(<SkeletonTable live={false} />);
    expect(container.firstChild).toHaveAttribute('aria-hidden', 'true');
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('SkeletonStat renders without crashing', () => {
    const { container } = render(<SkeletonStat />);
    expect(container.firstChild).toBeInTheDocument();
  });
});
