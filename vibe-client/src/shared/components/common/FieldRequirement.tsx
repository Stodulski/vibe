import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * Inline "(requerido)"/"(opcional)" suffix for a `FormField` label — mark
 * both required and optional fields so nobody has to guess (Practical UI
 * pp.339-343). Word markers avoid a top-of-form "fields marked with *"
 * instruction and avoid colouring an asterisk red.
 */
export function FieldRequirement({ required }: { required: boolean }) {
  return <span className="text-text-tertiary"> ({required ? t.common.requiredMarker : t.common.optional})</span>;
}
