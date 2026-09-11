import { render, screen, fireEvent } from '@testing-library/react';
import { PublicLinkBar } from './PublicLinkBar';

describe('PublicLinkBar', () => {
  it('calls onCopy when the copy button is pressed', () => {
    const onCopy = vi.fn();
    render(<PublicLinkBar publicUrl="https://vibe.app/mi-club" copied={false} onCopy={onCopy} />);
    fireEvent.click(screen.getByRole('button', { name: /copiar enlace/i }));
    expect(onCopy).toHaveBeenCalledTimes(1);
  });

  // The copy button's label is `hidden sm:inline` — an icon-only button on
  // mobile with no accessible name, before this fix. This checks the fix's
  // actual shape (an always-present `.sr-only` twin), since jsdom doesn't
  // apply the stylesheet that would otherwise hide the visible span here.
  it('keeps an always-visible sr-only label on the icon-only mobile button', () => {
    render(<PublicLinkBar publicUrl="https://vibe.app/mi-club" copied={false} onCopy={vi.fn()} />);
    const button = screen.getByRole('button', { name: /copiar enlace/i });
    expect(button.querySelector('.sr-only')).not.toBeNull();
  });
});
