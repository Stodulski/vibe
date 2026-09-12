import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import {
  SkeletonPage,
  SkeletonDashboard,
  SkeletonSettings,
  SkeletonBookings,
  SkeletonComplexSelector,
  SkeletonCancelInfo,
  SkeletonBookConfirm,
  SkeletonBookSuccess,
} from './Skeletons';

describe('Skeleton pages', () => {
  it('SkeletonPage renders exactly one status role (nested SkeletonTable stays aria-hidden)', () => {
    render(<SkeletonPage />);
    expect(screen.getAllByRole('status')).toHaveLength(1);
  });

  it('SkeletonDashboard renders with status role', () => {
    render(<SkeletonDashboard />);
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('SkeletonSettings renders with status role', () => {
    render(<SkeletonSettings />);
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('SkeletonBookings renders with status role', () => {
    render(<SkeletonBookings />);
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('SkeletonComplexSelector renders with status role', () => {
    render(<SkeletonComplexSelector />);
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('SkeletonCancelInfo renders with status role', () => {
    render(<SkeletonCancelInfo />);
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('SkeletonBookConfirm renders with status role', () => {
    render(<SkeletonBookConfirm />);
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('SkeletonBookSuccess renders with status role', () => {
    render(<SkeletonBookSuccess />);
    expect(screen.getByRole('status')).toBeInTheDocument();
  });
});
