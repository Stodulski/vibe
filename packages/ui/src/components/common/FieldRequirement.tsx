import { messages } from '../../messages';

interface FieldRequirementProps {
  required: boolean;
  /** Text for a required field. Defaults to "requerido". */
  requiredText?: string | undefined;
  /** Text for an optional field. Defaults to "opcional". */
  optionalText?: string | undefined;
}

/**
 * Inline "(requerido)"/"(opcional)" suffix for a `FormField` label — mark
 * both required and optional fields so nobody has to guess (Practical UI
 * pp.339-343). Word markers avoid a top-of-form "fields marked with *"
 * instruction and avoid colouring an asterisk red.
 */
export function FieldRequirement({
  required,
  requiredText = messages.requiredMarker,
  optionalText = messages.optional,
}: FieldRequirementProps) {
  return <span className="text-text-tertiary"> ({required ? requiredText : optionalText})</span>;
}
