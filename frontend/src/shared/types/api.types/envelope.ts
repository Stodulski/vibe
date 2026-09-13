import type { Spec } from './spec';

// ─── Envelope pattern (matches backend) ───

/**
 * The cursor-paginated list shell. `metadata` is the document's own
 * `Metadata` schema — the four list endpoints all repeat it.
 */
export interface PaginatedResponse<T> {
  data: T[];
  metadata: Spec<'Metadata'>;
}
