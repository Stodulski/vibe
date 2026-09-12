import * as React from 'react';
import { cn } from '@/shared/lib/utils';
import {
  AlertDialog as UiAlertDialog,
  AlertDialogContent as UiAlertDialogContent,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogOverlay,
  AlertDialogPortal,
  AlertDialogTitle,
  AlertDialogTrigger,
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

/**
 * Not a shadcn primitive — an app-specific slot for an icon/illustration
 * above an alert dialog's header. `ui/alert-dialog.tsx`'s header/title
 * styling already accounts for a sibling `[data-slot=alert-dialog-media]`;
 * the slot itself lives here so `ui/alert-dialog.tsx` stays shadcn plus
 * tokens (UI-04). Currently unused by any dialog in the app.
 */
function AlertDialogMedia({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="alert-dialog-media"
      className={cn(
        "bg-muted mb-2 inline-flex size-16 items-center justify-center rounded-md sm:group-data-[size=default]/alert-dialog-content:row-span-2 *:[svg:not([class*='size-'])]:size-8",
        className,
      )}
      {...props}
    />
  );
}

export {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogMedia,
  AlertDialogOverlay,
  AlertDialogPortal,
  AlertDialogTitle,
  AlertDialogTrigger,
};
