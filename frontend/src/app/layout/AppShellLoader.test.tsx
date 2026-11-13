import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { AppShellLoader } from './AppShellLoader';

describe('AppShellLoader', () => {
  it('announces loading once and draws the sidebar rail with one bar per nav item', () => {
    const { container } = render(<AppShellLoader />);
    expect(screen.getAllByRole('status')).toHaveLength(1);
    const rail = container.querySelector('.bg-bg-sidebar');
    expect(rail).not.toBeNull();
    expect(rail?.querySelectorAll('.space-y-0\\.5 > div')).toHaveLength(7);
  });
});
