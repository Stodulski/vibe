import { clsx, type ClassValue } from 'clsx';
import { extendTailwindMerge } from 'tailwind-merge';

// tailwind-merge only recognizes conflicts within its built-in class groups,
// so it doesn't know the custom @theme font-size tokens (styles/tokens.css)
// belong to the same group as text-sm/text-lg/etc. Without this, passing
// e.g. text-nav as a className into a primitive that already ships its own
// text-sm never dedupes, and which class wins becomes a source-order accident.
// Kept identical to the app's own `cn` so both sides merge the same way.
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
