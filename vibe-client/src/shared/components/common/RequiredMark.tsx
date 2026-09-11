import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * The asterisk that marks a required field.
 *
 * Not red. Red reads as an error, and this is a fact about the field, not a
 * complaint about what is in it. The form carries a line above it explaining
 * what the asterisk means, because an asterisk alone is a convention people
 * mostly know and sometimes do not.
 *
 * `aria-hidden` on the glyph with the word beside it in a screen-reader-only
 * span: "asterisk" read aloud after every label is noise, "requerido" is not.
 */
export function RequiredMark() {
  return (
    <>
      <span aria-hidden="true" className="text-text-tertiary">
        *
      </span>
      <span className="sr-only">{t.common.requiredMarker}</span>
    </>
  );
}
