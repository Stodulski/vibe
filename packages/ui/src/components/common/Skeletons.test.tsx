import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SkeletonTable } from './Skeletons';

describe('Skeleton primitives', () => {
  it('SkeletonTable with live={false} is aria-hidden and has no status role', () => {
    const { container } = render(<SkeletonTable live={false} />);
    expect(container.firstChild).toHaveAttribute('aria-hidden', 'true');
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });
});
