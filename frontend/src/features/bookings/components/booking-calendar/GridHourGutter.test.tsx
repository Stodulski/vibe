import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { GridHourGutter } from './GridHourGutter';

describe('GridHourGutter', () => {
  it('labels every hour of the day, midnight included', () => {
    render(<GridHourGutter />);

    expect(screen.getByText('00:00')).toBeInTheDocument();
    expect(screen.getByText('01:00')).toBeInTheDocument();
    expect(screen.getByText('23:00')).toBeInTheDocument();
    expect(screen.queryByText('24:00')).not.toBeInTheDocument();
  });
});
