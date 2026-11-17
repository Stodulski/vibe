import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { CategoryTabs } from './CategoryTabs';

const three = [
  { value: 'a', label: 'Caballeros 6ta' },
  { value: 'b', label: 'Damas 5ta' },
  { value: 'c', label: 'Mixto 4ta · Cancelada', muted: true },
];

describe('CategoryTabs', () => {
  it('wraps its underline tabs onto more lines, never scrolling sideways', () => {
    render(<CategoryTabs items={three} active="a" onChange={vi.fn()} ariaLabel="Categorías" />);

    const tab = screen.getByRole('button', { name: 'Caballeros 6ta' });
    const strip = tab.parentElement;
    if (!strip) throw new Error('the tab has no strip');
    expect(strip).toHaveClass('flex-wrap');
    expect(strip.className).not.toMatch(/overflow-x/);
  });

  it('keeps each tab label on one line and cuts a label wider than the screen with an ellipsis', () => {
    render(<CategoryTabs items={three} active="a" onChange={vi.fn()} ariaLabel="Categorías" />);

    const tab = screen.getByRole('button', { name: 'Caballeros 6ta' });
    expect(tab).toHaveClass('whitespace-nowrap', 'max-w-full');
    expect(tab.querySelector('span')).toHaveClass('min-w-0', 'truncate');
    expect(tab).toHaveAttribute('title', 'Caballeros 6ta');
  });

  it('becomes a select with a visible label from four categories, and no tabs', () => {
    const four = [...three, { value: 'd', label: 'Caballeros 7ma' }];
    render(<CategoryTabs items={four} active="a" onChange={vi.fn()} ariaLabel="Categorías" />);

    expect(screen.getByRole('combobox', { name: 'Categorías' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Damas 5ta' })).not.toBeInTheDocument();
  });
});
