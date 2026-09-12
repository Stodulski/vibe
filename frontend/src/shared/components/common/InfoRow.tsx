import type { ReactNode } from 'react';

/**
 * One labelled fact in a detail drawer: a small muted label with its value
 * underneath. Bare on purpose — the boxed-icon treatment these rows used to
 * carry made every fact look like a button.
 *
 * Shared rather than owned by the bookings feature: the booking, client and
 * blocked-slot drawers all read the same way, and a feature reaching into
 * another feature's folder for it is exactly what `src/shared` is for.
 */
export function InfoRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <p className="text-text-tertiary text-xs font-medium whitespace-nowrap">{label}</p>
      <div className="text-text-primary mt-0.5 text-sm">{children}</div>
    </div>
  );
}
