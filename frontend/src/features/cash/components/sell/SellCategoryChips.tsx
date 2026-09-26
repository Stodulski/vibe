import { Chip, ChipList } from '@/shared/components/common/ChipList';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface SellCategoryChipsProps {
  categories: string[];
  selected: string | null;
  onSelect: (category: string | null) => void;
}

/**
 * "Todas" plus one chip per category present in the active catalog
 * (`odd/tasks/pos-cashbox.md` T5b), in one horizontally scrolling row rather
 * than wrapping lines, so a long category list never pushes the products
 * down. Same scroller as the public DateSelector (`scrollbar-none`, snap) —
 * shared with the products feature's category picker via `ChipList`/`Chip`.
 */
export function SellCategoryChips({ categories, selected, onSelect }: SellCategoryChipsProps) {
  return (
    <ChipList aria-label={t.products.categoryLabel}>
      <Chip
        selected={selected === null}
        onClick={() => {
          onSelect(null);
        }}
      >
        {t.cash.sellAllCategories}
      </Chip>
      {categories.map((category) => (
        <Chip
          key={category}
          selected={selected === category}
          onClick={() => {
            onSelect(category);
          }}
        >
          {category}
        </Chip>
      ))}
    </ChipList>
  );
}
