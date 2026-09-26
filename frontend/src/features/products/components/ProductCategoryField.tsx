import { FormField } from '@/shared/components/common/FormField';
import { Input } from '@/shared/components/ui/input';
import { ES_AR } from '@/shared/i18n/es_AR';
import { buildCategoryChipOptions } from '../lib/categoryChipOptions';
import { ProductCategoryChips } from './ProductCategoryChips';

const t = ES_AR;

interface ProductCategoryFieldProps {
  value: string;
  onChange: (value: string) => void;
  error?: string | undefined;
  /** Categories already used across the catalog — offered as chips, never enforced (free text, per `productsCreate`'s own schema). */
  suggestions: readonly string[];
}

/**
 * Free-text category with a chip slider underneath: the catalog's own
 * categories first, then starter categories for a brand-new catalog with
 * nothing to suggest yet, deduplicated case-insensitively
 * (`buildCategoryChipOptions`). Tapping a chip fills the input; tapping the
 * chip matching the current value again clears it. Typing a category with no
 * chip still works — the field stays free text (60 chars max, DB-side).
 *
 * A native `<datalist>` used to back this instead, but its suggestions were
 * invisible with no affordance and poor on phones
 * (`odd/tasks/product-category-chips.md`) — the chip slider reuses
 * `SellCategoryChips`'s own scroll-snap pattern so Productos and Vender look
 * and behave the same.
 */
export function ProductCategoryField({ value, onChange, error, suggestions }: ProductCategoryFieldProps) {
  const chipOptions = buildCategoryChipOptions(suggestions);
  // `FormField` only auto-wires `aria-invalid`/`aria-describedby` when it is
  // handed a single control with no children of its own — this renders the
  // `Input` alongside a sibling chip row, so those two go on by hand.
  const errorId = 'product-category-error';

  return (
    <FormField label={t.products.categoryField} htmlFor="product-category" error={error}>
      <Input
        id="product-category"
        value={value}
        maxLength={60}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? errorId : undefined}
        onChange={(e) => {
          onChange(e.target.value);
        }}
      />
      {chipOptions.length > 0 && (
        <ProductCategoryChips
          categories={chipOptions}
          value={value}
          onSelect={onChange}
          ariaLabel={t.products.categoryChipsLabel}
        />
      )}
    </FormField>
  );
}
