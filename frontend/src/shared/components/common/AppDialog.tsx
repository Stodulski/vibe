import * as React from 'react';
import {
  Dialog as UiDialog,
  DialogContent as UiDialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/shared/components/ui/dialog';
import { useFocusRestoreOnClose } from '@/shared/hooks/useFocusRestoreOnClose';

const DialogFocusReturnContext = React.createContext<HTMLElement | null>(null);

/**
 * This app's `Dialog`: the shadcn primitive in `ui/dialog.tsx` plus the one
 * behavior that primitive should not own — restoring focus to whatever was
 * focused before opening, for the common case here where a dialog is opened
 * via an `open`/`onClose` prop pair rather than `<DialogTrigger>` (see
 * `useFocusRestoreOnClose`). `ui/dialog.tsx` stays shadcn plus tokens; this
 * is where the app-specific piece lives instead (UI-04).
 */
function Dialog({ open, ...props }: React.ComponentProps<typeof UiDialog>) {
  const lastFocused = useFocusRestoreOnClose(open);
  return (
    <DialogFocusReturnContext.Provider value={lastFocused}>
      <UiDialog {...(open !== undefined ? { open } : {})} {...props} />
    </DialogFocusReturnContext.Provider>
  );
}

function DialogContent({ onCloseAutoFocus, ...props }: React.ComponentProps<typeof UiDialogContent>) {
  const lastFocused = React.useContext(DialogFocusReturnContext);
  return (
    <UiDialogContent
      onCloseAutoFocus={(event) => {
        onCloseAutoFocus?.(event);
        if (!event.defaultPrevented && lastFocused) {
          event.preventDefault();
          lastFocused.focus();
        }
      }}
      {...props}
    />
  );
}

export { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle };
