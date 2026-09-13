import type { Spec } from './spec';

// ─── Envelope pattern (matches backend) ───

/**
 * The single-key envelope most endpoints answer with (`{ booking: … }`,
 * `{ courts: … }`). A client-side shorthand rather than a document schema:
 * `openapi.yaml` declares each envelope inline on its own operation, which
 * is where the other types in this folder read theirs from.
 */
export type ApiResponse<T> = Record<string, T>;

/**
 * The cursor-paginated list shell. `metadata` is the document's own
 * `Metadata` schema — the four list endpoints all repeat it.
 */
export interface PaginatedResponse<T> {
  data: T[];
  metadata: Spec<'Metadata'>;
}
