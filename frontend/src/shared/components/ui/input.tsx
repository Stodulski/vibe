import * as React from 'react';

import { cn } from '@/shared/lib/utils';

function Input({ className, type, ...props }: React.ComponentProps<'input'>) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        'border-border-interactive bg-bg-base/60 transition-input selection:bg-primary selection:text-primary-foreground file:text-foreground placeholder:text-text-tertiary/70 h-10 w-full min-w-0 rounded-xl border px-3.5 py-2 text-sm shadow-xs outline-none file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50',
        'hover:border-border-interactive-hover hover:bg-bg-base/80',
        'focus-visible:border-primary-500/60 focus-visible:bg-bg-base focus-visible:ring-primary-500/15 focus-visible:ring-[3px]',
        'aria-invalid:border-error-border aria-invalid:bg-error-bg/30 aria-invalid:ring-error-text/10 aria-invalid:focus-visible:border-error-border aria-invalid:focus-visible:ring-error-text/15 aria-invalid:ring-1 aria-invalid:focus-visible:ring-[3px]',
        className,
      )}
      {...props}
    />
  );
}

export { Input };
