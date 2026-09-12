import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

// The canvas renderer is not what this file is about, and happy-dom has no
// 2D context to give it.
vi.mock('./CanvasAreaChart', () => ({
  CanvasAreaChart: ({ data }: { data: { date: string; amount: number }[] }) => (
    <div data-testid="canvas-chart">{data.length}</div>
  ),
}));

import { ChartBody } from './ChartBody';

describe('ChartBody', () => {
  it('shows the skeleton while loading', () => {
    const { container } = render(<ChartBody state={{ status: 'loading' }} period="week" />);
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument();
    expect(screen.queryByTestId('canvas-chart')).not.toBeInTheDocument();
  });

  it('offers the retry carried by the error state', async () => {
    const onRetry = vi.fn();
    render(<ChartBody state={{ status: 'error', onRetry }} period="week" />);

    await userEvent.click(screen.getByRole('button', { name: 'Reintentar' }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it('shows the empty message when the ready state carries no points', () => {
    render(<ChartBody state={{ status: 'ready', chartData: [] }} period="week" />);
    expect(screen.getByText('No se encontraron resultados')).toBeInTheDocument();
    expect(screen.queryByTestId('canvas-chart')).not.toBeInTheDocument();
  });

  it('draws the chart and labels it by period when ready', () => {
    const chartData = [
      { date: '01/03', amount: 1000 },
      { date: '02/03', amount: 2000 },
    ];
    const { rerender } = render(<ChartBody state={{ status: 'ready', chartData }} period="week" />);
    expect(screen.getByTestId('canvas-chart')).toHaveTextContent('2');
    const weekLabel = screen.getByRole('img').getAttribute('aria-label');

    rerender(<ChartBody state={{ status: 'ready', chartData }} period="month" />);
    expect(screen.getByRole('img').getAttribute('aria-label')).not.toBe(weekLabel);
  });
});
