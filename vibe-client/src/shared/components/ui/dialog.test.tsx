import { useState } from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Dialog, DialogContent, DialogTitle } from './dialog';

/**
 * Regression test for M9: `document.activeElement as HTMLElement` used to
 * trust the cast at runtime too, so a focused non-HTMLElement (e.g. an SVG
 * icon with `tabIndex`) was stored as "last focused" and had `.focus()`
 * called on it again when the dialog closed. `instanceof HTMLElement`
 * narrows it to `null` instead, so close no longer tries to refocus it.
 *
 * Opening is triggered by a keydown ON the SVG itself (not a click on a
 * separate button) so `document.activeElement` is still the SVG at the exact
 * render where `Dialog` captures it — a click on another element would move
 * focus there first and never exercise the non-HTMLElement path at all.
 */
function ControlledDialog() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <svg
        data-testid="svg-focus-target"
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === 'Enter') setOpen(true);
        }}
      />
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogTitle>Title</DialogTitle>
          <button
            type="button"
            onClick={() => {
              setOpen(false);
            }}
          >
            Close from inside
          </button>
        </DialogContent>
      </Dialog>
    </>
  );
}

describe('Dialog focus return', () => {
  it('does not try to refocus a non-HTMLElement that was active before opening', async () => {
    const user = userEvent.setup();
    render(<ControlledDialog />);

    const svg = screen.getByTestId('svg-focus-target');
    svg.focus();
    expect(document.activeElement).toBe(svg);
    const focusSpy = vi.spyOn(svg, 'focus');

    fireEvent.keyDown(svg, { key: 'Enter' });
    await screen.findByText('Title');

    await user.click(screen.getByRole('button', { name: 'Close from inside' }));

    await waitFor(() => {
      expect(screen.queryByText('Title')).not.toBeInTheDocument();
    });
    expect(focusSpy).not.toHaveBeenCalled();
  });
});
