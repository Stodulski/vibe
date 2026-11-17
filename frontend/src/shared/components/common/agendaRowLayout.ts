import { cn } from '@/shared/lib/utils';

/** The one column of rows, a thin divider between them and no card around them. */
export const AGENDA_LIST_CLASS = 'divide-border-subtle divide-y';

/** The layout of one row, shared by `AgendaRow` and its skeleton so a change touches both. */
export const AGENDA_ROW_LAYOUT_CLASS =
  'flex min-h-14 w-full flex-col gap-3 rounded-xl px-3 py-4 text-left sm:flex-row sm:items-center sm:gap-5';

/** The whole row as its focusable, pressable target: a button or a link wraps `AgendaRow` with it. */
export const AGENDA_ROW_CLASS = cn(
  AGENDA_ROW_LAYOUT_CLASS,
  'focus-self hover:bg-bg-subtle cursor-pointer transition-colors',
);
