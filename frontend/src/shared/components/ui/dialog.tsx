'use client';

import * as React from 'react';
import { XIcon } from 'lucide-react';
import { Dialog as DialogPrimitive } from 'radix-ui';

import { cn } from '@/shared/lib/utils';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

// Most of this app's dialogs are opened via an `open`/`onClose` prop pair
// from an ancestor page, not via `<DialogTrigger>` — so Radix's own
// `context.triggerRef` (populated only by `DialogTrigger`) stays null and its
// built-in "focus returns to the trigger on close" behavior silently does
// nothing. This context/ref remembers whatever was focused right before the
// dialog opened (almost always the button that opened it) so `DialogContent`
// can restore focus there itself on close, matching Radix's documented
// default instead of leaving focus on `<body>`.
const DialogFocusReturnContext = React.createContext<HTMLElement | null>(null);

function Dialog({ open, ...props }: React.ComponentProps<typeof DialogPrimitive.Root>) {
  // "Adjusting state during render" — React's own escape hatch for state that
  // depends on the exact instant a prop changed, not just its current value.
  // A `useEffect` would run too late here: Radix's own focus-trap effect
  // (which moves focus into the dialog) fires before ours, so by the time we
  // read it focus has already left the trigger.
  const [wasOpen, setWasOpen] = React.useState(false);
  const [lastFocused, setLastFocused] = React.useState<HTMLElement | null>(null);
  if (open && !wasOpen) {
    setWasOpen(true);
    const activeElement = document.activeElement;
    setLastFocused(activeElement instanceof HTMLElement ? activeElement : null);
  } else if (!open && wasOpen) {
    setWasOpen(false);
  }
  return (
    <DialogFocusReturnContext.Provider value={lastFocused}>
      <DialogPrimitive.Root data-slot="dialog" {...(open !== undefined ? { open } : {})} {...props} />
    </DialogFocusReturnContext.Provider>
  );
}

function DialogTrigger({ ...props }: React.ComponentProps<typeof DialogPrimitive.Trigger>) {
  return <DialogPrimitive.Trigger data-slot="dialog-trigger" {...props} />;
}

function DialogPortal({ ...props }: React.ComponentProps<typeof DialogPrimitive.Portal>) {
  return <DialogPrimitive.Portal data-slot="dialog-portal" {...props} />;
}

function DialogClose({ ...props }: React.ComponentProps<typeof DialogPrimitive.Close>) {
  return <DialogPrimitive.Close data-slot="dialog-close" {...props} />;
}

function DialogOverlay({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Overlay>) {
  return (
    <DialogPrimitive.Overlay
      data-slot="dialog-overlay"
      className={cn(
        'data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0 fixed inset-0 z-50 bg-black/50',
        className,
      )}
      {...props}
    />
  );
}

function DialogContent({
  className,
  children,
  showCloseButton = true,
  onCloseAutoFocus,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Content> & {
  showCloseButton?: boolean;
}) {
  const lastFocused = React.useContext(DialogFocusReturnContext);
  return (
    <DialogPortal data-slot="dialog-portal">
      <DialogOverlay />
      <DialogPrimitive.Content
        aria-describedby={undefined}
        aria-modal="true"
        data-slot="dialog-content"
        className={cn(
          'bg-background data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0 fixed z-50 flex w-full flex-col gap-4 overflow-y-auto border p-5 shadow-lg duration-200 outline-none',
          'data-[state=closed]:sm:zoom-out-95 data-[state=open]:sm:zoom-in-95 inset-0 rounded-none pb-[calc(1.5rem+env(safe-area-inset-bottom,0px))] sm:inset-auto sm:top-[50%] sm:left-[50%] sm:max-h-[calc(100dvh-3rem)] sm:max-w-lg sm:translate-x-[-50%] sm:translate-y-[-50%] sm:rounded-lg sm:p-6 [&>form]:flex [&>form]:flex-1 [&>form]:flex-col sm:[&>form]:flex-none [&>form>:last-child]:mt-auto sm:[&>form>:last-child]:mt-0',
          className,
        )}
        onCloseAutoFocus={(event) => {
          onCloseAutoFocus?.(event);
          if (!event.defaultPrevented && lastFocused) {
            event.preventDefault();
            lastFocused.focus();
          }
        }}
        {...props}
      >
        {children}
        {showCloseButton && (
          <DialogPrimitive.Close
            data-slot="dialog-close"
            className="ring-offset-background focus:ring-ring data-[state=open]:bg-accent data-[state=open]:text-muted-foreground absolute top-4 right-4 rounded-xs opacity-70 transition-opacity hover:opacity-100 focus:ring-2 focus:ring-offset-2 focus:outline-hidden disabled:pointer-events-none [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"
          >
            <XIcon />
            <span className="sr-only">{t.common.close}</span>
          </DialogPrimitive.Close>
        )}
      </DialogPrimitive.Content>
    </DialogPortal>
  );
}

function DialogHeader({ className, ...props }: React.ComponentProps<'div'>) {
  return <div data-slot="dialog-header" className={cn('flex flex-col gap-2 pr-8 text-left', className)} {...props} />;
}

function DialogFooter({
  className,
  showCloseButton = false,
  children,
  ...props
}: React.ComponentProps<'div'> & {
  showCloseButton?: boolean;
}) {
  return (
    <div
      data-slot="dialog-footer"
      className={cn('flex flex-col-reverse gap-2 sm:flex-row sm:justify-end', className)}
      {...props}
    >
      {children}
      {showCloseButton && (
        <DialogPrimitive.Close asChild>
          <Button variant="outline">{t.common.close}</Button>
        </DialogPrimitive.Close>
      )}
    </div>
  );
}

function DialogTitle({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Title>) {
  return (
    <DialogPrimitive.Title
      data-slot="dialog-title"
      className={cn('text-lg leading-none font-semibold', className)}
      {...props}
    />
  );
}

function DialogDescription({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Description>) {
  return (
    <DialogPrimitive.Description
      data-slot="dialog-description"
      className={cn('text-muted-foreground text-sm', className)}
      {...props}
    />
  );
}

export {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
};
