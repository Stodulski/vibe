import { Button } from '@/shared/components/ui/button';
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
 * down. Same scroller as the public DateSelector (`scrollbar-none`, snap).
 */
export function SellCategoryChips({ categories, selected, onSelect }: SellCategoryChipsProps) {
  return (
    <div
      className="flex snap-x scrollbar-none gap-1.5 overflow-x-auto pb-1"
      role="group"
      aria-label={t.products.categoryLabel}
    >
      <Button
        type="button"
        size="sm"
        className="shrink-0 snap-start"
        variant={selected === null ? 'default' : 'outline'}
        aria-pressed={selected === null}
        onClick={() => {
          onSelect(null);
        }}
      >
        {t.cash.sellAllCategories}
      </Button>
      {categories.map((category) => (
        <Button
          key={category}
          type="button"
          size="sm"
          className="shrink-0 snap-start"
          variant={selected === category ? 'default' : 'outline'}
          aria-pressed={selected === category}
          onClick={() => {
            onSelect(category);
          }}
        >
          {category}
        </Button>
      ))}
    </div>
  );
}
