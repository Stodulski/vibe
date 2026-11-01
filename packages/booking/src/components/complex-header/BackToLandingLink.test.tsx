import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { BackToLandingLink } from './BackToLandingLink';

function setReferrer(value: string) {
  vi.spyOn(document, 'referrer', 'get').mockReturnValue(value);
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('BackToLandingLink', () => {
  it('links back to the landing page the visitor came from', () => {
    setReferrer(`${window.location.origin}/canchas/rosario`);
    render(<BackToLandingLink />);

    expect(screen.getByRole('link', { name: 'Volver' })).toHaveAttribute('href', '/canchas/rosario');
  });

  it('goes back in history when there is history to go back to', () => {
    setReferrer(`${window.location.origin}/canchas/rosario`);
    vi.spyOn(window.history, 'length', 'get').mockReturnValue(2);
    const back = vi.spyOn(window.history, 'back').mockImplementation(() => undefined);
    render(<BackToLandingLink />);

    fireEvent.click(screen.getByRole('link', { name: 'Volver' }));

    expect(back).toHaveBeenCalledOnce();
  });

  it('renders nothing for a visitor who arrived from another site', () => {
    setReferrer('https://www.google.com/');
    render(<BackToLandingLink />);

    expect(screen.queryByRole('link', { name: 'Volver' })).toBeNull();
  });
});
