import * as React from 'react';
import {
  AlertDialog as UiAlertDialog,
  AlertDialogContent as UiAlertDialogContent,
  AlertDialogAction,
  AlertDialogTitle,
} from '@/shared/components/ui/alert-dialog';
import { useFocusRestoreOnClose } from '@/shared/hooks/useFocusRestoreOnClose';

const AlertDialogFocusReturnContext = React.createContext<HTMLElement | null>(null);

/**
 * This app's `AlertDialog` — same rationale as `AppDialog` (`./AppDialog`):
 * `ui/alert-dialog.tsx` stays the shadcn primitive plus tokens, and the
 * focus-restore behavior for a dialog opened via an `open`/`onClose` prop
 * pair lives here instead (UI-04).
 */
function AlertDialog({ open, ...props }: React.ComponentProps<typeof UiAlertDialog>) {
  const lastFocused = useFocusRestoreOnClose(open);
  return (
    <AlertDialogFocusReturnContext.Provider value={lastFocused}>
      <UiAlertDialog {...(open !== undefined ? { open } : {})} {...props} />
    </AlertDialogFocusReturnContext.Provider>
  );
}

function AlertDialogContent({ onCloseAutoFocus, ...props }: React.ComponentProps<typeof UiAlertDialogContent>) {
  const lastFocused = React.useContext(AlertDialogFocusReturnContext);
  return (
    <UiAlertDialogContent
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

export { AlertDialog, AlertDialogAction, AlertDialogContent, AlertDialogTitle };
