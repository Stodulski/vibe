import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { CommandDialog, CommandInput, CommandList, CommandEmpty } from './command';

// Radix's Dialog.Root renders whatever children it's given unconditionally —
// only DialogContent (via its own Presence) is gated by the open state. The
// old CommandDialog placed DialogHeader/DialogTitle as a sibling of
// DialogContent, so the sr-only "title" text sat in the DOM even while the
// palette was closed. It now lives inside DialogContent instead, so it
// should only be present once the dialog is actually open.
describe('CommandDialog', () => {
  it('does not render the title in the DOM while closed', () => {
    render(
      <CommandDialog
        open={false}
        onOpenChange={() => {
          /* noop */
        }}
        title="Paleta de comandos"
        description="Buscar un comando"
      >
        <CommandInput />
        <CommandList>
          <CommandEmpty>Sin resultados</CommandEmpty>
        </CommandList>
      </CommandDialog>,
    );

    expect(screen.queryByText('Paleta de comandos')).not.toBeInTheDocument();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('renders the (visually hidden) title once the dialog is open', () => {
    render(
      <CommandDialog
        open
        onOpenChange={() => {
          /* noop */
        }}
        title="Paleta de comandos"
        description="Buscar un comando"
      >
        <CommandInput />
        <CommandList>
          <CommandEmpty>Sin resultados</CommandEmpty>
        </CommandList>
      </CommandDialog>,
    );

    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText('Paleta de comandos')).toBeInTheDocument();
  });
});
