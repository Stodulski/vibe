import * as React from 'react';

import { cn } from '@/shared/lib/utils';

function Textarea({ className, ...props }: React.ComponentProps<'textarea'>) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        'border-border-interactive bg-bg-base/60 transition-input placeholder:text-text-tertiary/70 flex field-sizing-content min-h-16 w-full rounded-xl border px-3.5 py-2.5 text-sm shadow-xs outline-none disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50',
        'hover:border-border-interactive-hover hover:bg-bg-base/80',
        'focus-visible:border-primary-500/60 focus-visible:bg-bg-base focus-visible:ring-primary-500/15 focus-visible:ring-[3px]',
        'aria-invalid:border-error-border aria-invalid:bg-error-bg/30 aria-invalid:ring-error-text/10 aria-invalid:focus-visible:border-error-border aria-invalid:focus-visible:ring-error-text/15 aria-invalid:ring-1 aria-invalid:focus-visible:ring-[3px]',
        className,
      )}
      {...props}
    />
  );
}

export { Textarea };
