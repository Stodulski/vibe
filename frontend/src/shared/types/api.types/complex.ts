import type { Body, Spec } from './spec';

// ─── Complex ───

/**
 * What a venue offers. A closed vocabulary — the server's
 * complexes_amenities_known CHECK has the matching CHECK constraint.
 *
 * Read off the update-complex request body, which is where `openapi.yaml`
 * enumerates the set; the `Complex` and `PublicComplex` response schemas type
 * the same field as a plain `string[]`. Deriving it from the one place that
 * spells it out keeps the union honest without restating it here.
 */
export type Amenity = NonNullable<Body<'complexesUpdate'>['amenities']>[number];

/**
 * `amenities` is narrowed to {@link Amenity}: the document types the response
 * field as `string[]` (see above), and every screen that renders an amenity
 * looks it up in a fixed icon/label table, so an unknown string would render
 * as a blank row.
 */
export type Complex = Omit<Spec<'Complex'>, 'amenities'> & { amenities: Amenity[] };

export type DayOfWeek = Spec<'Weekday'>;

export type Schedule = Spec<'Schedule'>;

export type CreateComplexRequest = Body<'complexesCreate'>;

export type UpdateComplexRequest = Body<'complexesUpdate'>;

export type UpdateSchedulesRequest = Body<'complexesUpdateSchedules'>;

export type BlockedSlot = Spec<'BlockedSlot'>;
