import { createSortedRowModel, rowSortingFeature, tableFeatures } from '@tanstack/react-table';

/**
 * Shared TanStack Table v9 `features` configuration for this app's data tables.
 *
 * A data table here only needs the core row model (automatic) + row sorting — no
 * filtering, pagination, grouping, etc. — so they share one `features` object
 * instead of each declaring an equivalent one independently.
 *
 * `TFeatures` (the type param on `ColumnDef`/`Column`/`Table`/etc.) is derived from
 * `typeof appTableFeatures` at each call site.
 */
export const appTableFeatures = tableFeatures({
  rowSortingFeature,
  sortedRowModel: createSortedRowModel(),
});

export type AppTableFeatures = typeof appTableFeatures;
