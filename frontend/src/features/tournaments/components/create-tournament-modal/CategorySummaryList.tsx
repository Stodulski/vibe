import { Plus } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { categoryFormTitle, categorySummaryLine } from './categorySummary';
import type { CategoryFormValues } from '../../schemas/tournamentForm.schema';

const t = ES_AR.tournaments;

interface Props {
  categories: CategoryFormValues[];
  onEdit: (index: number) => void;
  onRemove: (index: number) => void;
  onAdd: () => void;
  error: string | undefined;
}

/** The saved categories as compact rows with "Editar" and "Quitar", and the button that adds one. */
export function CategorySummaryList({ categories, onEdit, onRemove, onAdd, error }: Props) {
  return (
    <div className="space-y-4">
      <h3 className="text-text-primary text-sm font-semibold">{t.createSteps.step2}</h3>
      {categories.length === 0 ? (
        <p className="text-text-tertiary text-sm">{t.createSteps.noCategories}</p>
      ) : (
        <ul className="border-border-subtle divide-border-subtle divide-y rounded-xl border px-3.5">
          {categories.map((category, index) => {
            const title = categoryFormTitle(category);
            return (
              <li key={index} aria-label={title} className="flex items-center justify-between gap-2 py-2.5">
                <div className="min-w-0">
                  <p className="text-text-primary truncate text-sm font-medium">{title}</p>
                  <p className="text-text-tertiary text-xs">{categorySummaryLine(category)}</p>
                </div>
                <div className="flex shrink-0 gap-1">
                  <Button
                    type="button"
                    variant="ghost"
                    size="lg"
                    className="px-3"
                    aria-label={t.createSteps.editCategoryLabel(title)}
                    onClick={() => {
                      onEdit(index);
                    }}
                  >
                    {t.createSteps.editCategory}
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="lg"
                    className="px-3"
                    aria-label={t.createSteps.removeCategoryLabel(title)}
                    onClick={() => {
                      onRemove(index);
                    }}
                  >
                    {t.createSteps.remove}
                  </Button>
                </div>
              </li>
            );
          })}
        </ul>
      )}
      {error && (
        <p role="alert" className="text-error-text text-xs">
          {error}
        </p>
      )}
      <Button type="button" variant="outline" className="w-full rounded-xl" onClick={onAdd}>
        <Plus className="size-4" aria-hidden="true" />
        {t.form.addCategory}
      </Button>
    </div>
  );
}
