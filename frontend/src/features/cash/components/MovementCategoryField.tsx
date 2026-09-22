import { FormField } from '@/shared/components/common/FormField';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { MovementCategory } from '../lib/movementCategories';

const t = ES_AR;

export function MovementCategoryField({
  categories,
  value,
  onChange,
  error,
}: {
  categories: readonly MovementCategory[];
  value: MovementCategory;
  onChange: (category: MovementCategory) => void;
  error?: string | undefined;
}) {
  return (
    <FormField label={t.cash.movementCategory} htmlFor="movement-category" error={error}>
      <Select
        value={value}
        onValueChange={(v) => {
          onChange(v as MovementCategory);
        }}
      >
        <SelectTrigger id="movement-category">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {categories.map((c) => (
            <SelectItem key={c} value={c}>
              {t.cash.categories[c]}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </FormField>
  );
}
