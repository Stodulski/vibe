import { clsx, type ClassValue } from 'clsx';
import { extendTailwindMerge } from 'tailwind-merge';

// tailwind-merge only recognizes conflicts within its built-in class groups,
// so it doesn't know these custom @theme font-size tokens (globals.css)
// belong to the same group as text-sm/text-lg/etc. Without this, passing
// e.g. text-nav as a className into a shadcn primitive that already ships
// its own text-sm never dedupes — both classes survive into the DOM, and
// which one wins becomes a coin flip decided by generated-CSS source order.
const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      'font-size': ['text-micro', 'text-nav', 'text-brand'],
    },
  },
});

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
