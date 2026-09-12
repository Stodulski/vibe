import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AppMasthead } from './AppMasthead';

// UI-11: the command palette's Cmd/Ctrl+K shortcut used to have no visible
// entry point. The masthead's search button dispatches the same
// `open-command-palette` event `CommandPalette` listens for — but only where
// a `<CommandPalette />` is actually mounted, gated by `showSearchTrigger`.
describe('AppMasthead search trigger', () => {
  it('does not render a search button by default', () => {
    render(<AppMasthead brand={<span>Brand</span>} />);
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('renders a search button when showSearchTrigger is set', () => {
    render(<AppMasthead brand={<span>Brand</span>} showSearchTrigger />);
    expect(screen.getByRole('button', { name: /buscar/i })).toBeInTheDocument();
  });

  it('dispatches open-command-palette when clicked', async () => {
    const user = userEvent.setup();
    const handler = vi.fn();
    window.addEventListener('open-command-palette', handler);

    render(<AppMasthead brand={<span>Brand</span>} showSearchTrigger />);
    await user.click(screen.getByRole('button', { name: /buscar/i }));

    expect(handler).toHaveBeenCalledTimes(1);
    window.removeEventListener('open-command-palette', handler);
  });

  it('shows the keyboard shortcut hint', () => {
    render(<AppMasthead brand={<span>Brand</span>} showSearchTrigger />);
    expect(screen.getByText(/ctrl ?k|⌘k/i)).toBeInTheDocument();
  });
});
