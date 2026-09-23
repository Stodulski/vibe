import { useId } from 'react';
import { FormField } from '@/shared/components/common/FormField';
import { Input } from '@/shared/components/ui/input';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface ProductCategoryFieldProps {
  value: string;
  onChange: (value: string) => void;
  error?: string | undefined;
  /** Categories already used across the catalog — suggested, never enforced (free text, per `productsCreate`'s own schema). */
  suggestions: readonly string[];
}

/**
 * Free-text category with suggestions from categories already in use.
 *
 * The app has no combobox component (checked: no `Combobox`/`Command`
 * anywhere under `shared/components`), and building one is out of scope for
 * a single free-text field with soft suggestions — a native `<datalist>`
 * gives the same "type or pick from a list" affordance with no new
 * dependency and no extra a11y wiring (the browser owns the listbox).
 */
export function ProductCategoryField({ value, onChange, error, suggestions }: ProductCategoryFieldProps) {
  const listId = useId();
  // `FormField` only auto-wires `aria-invalid`/`aria-describedby` when it is
  // handed a single control with no children of its own — this renders the
  // `Input` alongside a sibling `<datalist>`, so those two go on by hand.
  const errorId = 'product-category-error';

  return (
    <FormField label={t.products.categoryField} htmlFor="product-category" error={error}>
      <Input
        id="product-category"
        list={listId}
        value={value}
        maxLength={60}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? errorId : undefined}
        onChange={(e) => {
          onChange(e.target.value);
        }}
      />
      <datalist id={listId}>
        {suggestions.map((category) => (
          <option key={category} value={category} />
        ))}
      </datalist>
    </FormField>
  );
}
