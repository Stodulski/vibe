import { Chip, ChipList } from '@/shared/components/common/ChipList';

interface ProductCategoryChipsProps {
  categories: readonly string[];
  value: string;
  onSelect: (value: string) => void;
  /** Own accessible name — distinct from the sibling `Input`'s label, so screen readers (and `getByLabelText`) tell the two apart. */
  ariaLabel: string;
}

/**
 * Chip picker for `ProductCategoryField`'s category input. The chip whose
 * label matches the current value (trimmed, case-insensitive) shows as
 * selected; tapping it again clears the field instead of leaving it stuck on
 * a category the person is trying to remove. Tapping any other chip fills
 * the input with its exact spelling.
 */
export function ProductCategoryChips({ categories, value, onSelect, ariaLabel }: ProductCategoryChipsProps) {
  const normalizedValue = value.trim().toLowerCase();

  return (
    <ChipList aria-label={ariaLabel}>
      {categories.map((category) => {
        const selected = normalizedValue !== '' && category.toLowerCase() === normalizedValue;
        return (
          <Chip
            key={category}
            selected={selected}
            onClick={() => {
              onSelect(selected ? '' : category);
            }}
          >
            {category}
          </Chip>
        );
      })}
    </ChipList>
  );
}
