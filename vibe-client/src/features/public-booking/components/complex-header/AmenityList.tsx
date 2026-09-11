import { orderedAmenities } from '@/shared/lib/amenities';

/**
 * What the venue offers.
 *
 * Icons carry a label rather than standing alone. A shower glyph and a locker
 * glyph are near-identical at 14px, and an amenity nobody can identify is
 * decoration — this is the one place on the page a player is scanning for a
 * specific fact ("do they have parking?").
 *
 * Renders nothing when the list is empty. The caller already hides the whole
 * section in that case; this stays as its own guard so the component cannot
 * produce an empty list if it is ever used somewhere else.
 *
 * No heading of its own: the accordion trigger above it is the heading.
 */
export function AmenityList({ amenities }: { amenities: readonly string[] }) {
  const listed = orderedAmenities(amenities);
  if (listed.length === 0) return null;

  return (
    // Chips that wrap: a pill per amenity, as many per row as fit, on every
    // viewport. A player scans for one word ("Estacionamiento"), and a
    // bounded pill reads as one fact at a glance where a bare label in a
    // sparse grid reads as a stray line. `whitespace-nowrap` keeps a pill
    // from breaking mid-label; the row wraps instead.
    <ul className="flex flex-wrap gap-2">
      {listed.map(({ value, label, icon: Icon }) => (
        <li
          key={value}
          // 12px on a phone, 14px on a desktop. The list is a secondary
          // fact about the club, read after the name and the hours.
          className="inline-flex items-center gap-1.5 whitespace-nowrap rounded-full border border-border-subtle bg-bg-elevated px-3 py-1.5 text-xs text-text-secondary lg:text-sm"
        >
          <Icon className="size-3.5 shrink-0 text-primary-400" aria-hidden="true" />
          <span>{label}</span>
        </li>
      ))}
    </ul>
  );
}
