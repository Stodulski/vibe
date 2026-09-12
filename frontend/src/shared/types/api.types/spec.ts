import type { Loose } from '@/shared/lib/apiParse';
import type { components, operations } from '../api.generated';

/**
 * The three ways this folder reads the generated OpenAPI types, so no other
 * file has to spell out `components['schemas'][…]` or walk an operation's
 * response tree by hand.
 *
 * Not re-exported from `src/shared/types/api.types.ts`: these are the seam
 * between the generated document and the names the app uses, and nothing
 * outside this folder should reach through them to the raw document. Ask for
 * `Booking`, not `Spec<'Booking'>`.
 *
 * Deriving instead of restating is the whole point — a field the backend
 * renames, retypes or drops in `openapi.yaml` becomes a compile error here
 * rather than a shape the client keeps believing in. Where a declaration
 * below narrows what the document says (an optional field the client treats
 * as guaranteed, a `string` the client treats as a closed union), it says so
 * and why; a bare alias means the document is the whole truth.
 */
export type Spec<K extends keyof components['schemas']> = components['schemas'][K];

/**
 * A closed vocabulary the document declares, reopened for reading.
 *
 * The server owns these lists and can grow one in a deploy the client has
 * not seen — a new payment method, a new sport. Parsing such a value as a
 * hard failure takes the whole response down: one court with an unknown
 * sport would empty the public slot grid, and one stored payment row with a
 * retired status would blank a booking's detail panel. So response schemas
 * accept any string and the type keeps the known members for autocomplete
 * and for `switch`/lookup exhaustiveness, while saying out loud that an
 * unknown one can arrive — every consumer of these fields already falls back
 * to rendering the raw value.
 *
 * Request bodies keep the closed union: sending an unknown member is the
 * client's own bug, and the server answers it with a 422 either way.
 */
export type Open<T extends string> = T | (string & {});

/** The `application/json` body an operation answers 2xx with. */
export type Ok<K extends keyof operations> =
  operations[K]['responses'] extends { 200: { content: { 'application/json': infer B } } }
    ? B
    : operations[K]['responses'] extends { 201: { content: { 'application/json': infer B } } }
      ? B
      : never;

/**
 * The `application/json` body an operation accepts, with its optional fields
 * widened to also allow an explicit `undefined`.
 *
 * Request bodies are assembled from `z.infer`-ed form DTOs, and Zod types
 * every `.optional()` field as `field?: X | undefined` — which under
 * `exactOptionalPropertyTypes` is not assignable to the document's
 * `field?: X`. `JSON.stringify` drops an explicitly-`undefined` value exactly
 * as it drops an absent key, so the two are the same request on the wire;
 * {@link Loose} says that in the type system. Responses go the other way and
 * are narrowed back with `exact` (see `src/shared/lib/apiParse.ts`).
 */
export type Body<K extends keyof operations> =
  NonNullable<operations[K]['requestBody']> extends { content: { 'application/json': infer B } } ? Loose<B> : never;
