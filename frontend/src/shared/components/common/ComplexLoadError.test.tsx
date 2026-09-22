import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ComplexLoadError } from './ComplexLoadError';

describe('ComplexLoadError', () => {
  it('calls onRetry when the button is clicked', () => {
    const onRetry = vi.fn();
    render(<ComplexLoadError onRetry={onRetry} />);

    screen.getByRole('button').click();
    expect(onRetry).toHaveBeenCalled();
  });

  it('disables the retry button while a retry is in flight, so it cannot be spammed', () => {
    const onRetry = vi.fn();
    render(<ComplexLoadError onRetry={onRetry} isFetching />);

    const button = screen.getByRole('button');
    expect(button).toBeDisabled();

    button.click();
    expect(onRetry).not.toHaveBeenCalled();
  });
});
