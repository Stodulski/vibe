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

/**
 * The two error bodies the document declares: `Error` is the bare
 * human-readable string every 4xx/5xx uses, `ValidationError` the
 * field-name-to-message map a 422 answers with.
 */
export type ErrorResponse = Spec<'Error'> | Spec<'ValidationError'>;
