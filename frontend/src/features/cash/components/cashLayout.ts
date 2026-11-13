/** Structure shared by the loaded cash views and their skeletons, so a layout change touches both. */
export const CASH_STACK_CLASS = 'max-w-3xl space-y-6';
export const EXPECTED_CASH_CARD_CLASS = 'w-full space-y-4';
export const CASH_ACTIONS_CLASS = 'grid grid-cols-2 gap-2 sm:flex sm:items-center';
/** The third action (Cerrar caja) spans both columns below `sm`, then sits at the end of the row. */
export const CASH_ACTION_LAST_CLASS = 'col-span-2 sm:ml-auto';
export const MOVEMENT_ROW_CLASS =
  'border-border-subtle flex items-start justify-between gap-4 border-b py-4 last:border-b-0';
