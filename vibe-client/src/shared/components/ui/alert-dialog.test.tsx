import { useState } from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AlertDialog, AlertDialogContent, AlertDialogTitle, AlertDialogAction } from './alert-dialog';

/**
 * Regression test for M9, mirroring `dialog.test.tsx`: a focused
 * non-HTMLElement must not be refocused when the alert dialog closes.
 * Opening happens via a keydown on the SVG itself so `document.activeElement`
 * is still the SVG at the render where `AlertDialog` captures it.
 */
function ControlledAlertDialog() {
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
      <AlertDialog open={open} onOpenChange={setOpen}>
        <AlertDialogContent>
          <AlertDialogTitle>Title</AlertDialogTitle>
          <AlertDialogAction
            onClick={() => {
              setOpen(false);
            }}
          >
            Confirmar
          </AlertDialogAction>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

describe('AlertDialog focus return', () => {
  it('does not try to refocus a non-HTMLElement that was active before opening', async () => {
    const user = userEvent.setup();
    render(<ControlledAlertDialog />);

    const svg = screen.getByTestId('svg-focus-target');
    svg.focus();
    expect(document.activeElement).toBe(svg);
    const focusSpy = vi.spyOn(svg, 'focus');

    fireEvent.keyDown(svg, { key: 'Enter' });
    await screen.findByText('Title');

    await user.click(screen.getByRole('button', { name: 'Confirmar' }));

    await waitFor(() => {
      expect(screen.queryByText('Title')).not.toBeInTheDocument();
    });
    expect(focusSpy).not.toHaveBeenCalled();
  });
});
