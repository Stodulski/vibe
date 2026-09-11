import * as React from 'react';
import { CheckIcon } from 'lucide-react';
import { Checkbox as CheckboxPrimitive } from 'radix-ui';

import { cn } from '@/shared/lib/utils';

function Checkbox({ className, ...props }: React.ComponentProps<typeof CheckboxPrimitive.Root>) {
  return (
    <CheckboxPrimitive.Root
      data-slot="checkbox"
      className={cn(
        // Glow color reuses the existing --color-primary-500 token (CSS
        // relative-color syntax) instead of repeating its RGB triplet, same
        // as SidebarNavLink/ConfirmDialogFooter.
        'peer size-[18px] shrink-0 rounded-[5px] border border-border-interactive bg-bg-base/60 shadow-xs outline-none transition-input hover:border-border-interactive-hover hover:bg-bg-base/80 disabled:cursor-not-allowed disabled:pointer-events-none disabled:opacity-50 focus-visible:border-primary-500/60 focus-visible:ring-[3px] focus-visible:ring-primary-500/15 aria-invalid:border-error-border data-[state=checked]:border-primary-500 data-[state=checked]:bg-primary-500 data-[state=checked]:text-primary-foreground data-[state=checked]:shadow-[0_0_8px_rgb(from_var(--color-primary-500)_r_g_b/20%)]',
        className,
      )}
      {...props}
    >
      <CheckboxPrimitive.Indicator
        data-slot="checkbox-indicator"
        className="flex items-center justify-center text-current transition-none"
      >
        <CheckIcon className="size-3.5" />
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  );
}

export { Checkbox };
