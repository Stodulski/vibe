import { useState } from 'react';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { type CategoryFormValues } from '../../schemas/tournamentForm.schema';
import { CategoryEditor, type CategoryView } from './CategoryEditor';
import { CategorySummaryList } from './CategorySummaryList';
import type { CreateTournamentForm } from './useCreateTournamentForm';

interface Editing {
  index: number;
  view: CategoryView;
  /** What the category held when the editor opened; null for a category that did not exist yet. */
  before: CategoryFormValues | null;
}

interface Props {
  flow: CreateTournamentForm;
  onBack: () => void;
  onNext: () => void;
}

/**
 * Step 2: the list of categories as summary rows. Adding or editing one opens its two views in
 * place of the list, so no view grows long. Leaving the first view with "Volver" drops a new
 * category and puts an edited one back as it was.
 */
export function CategoriesStep({ flow, onBack, onNext }: Props) {
  const { form } = flow;
  const [editing, setEditing] = useState<Editing | null>(null);
  const saved = form.watch('categories');
  const listError = form.formState.errors.categories?.root?.message ?? form.formState.errors.categories?.message;

  const open = (index: number, before: CategoryFormValues | null) => {
    setEditing({ index, view: 'category', before });
  };
  const discard = () => {
    if (!editing) return;
    if (editing.before) form.setValue(`categories.${String(editing.index)}` as `categories.${number}`, editing.before);
    else flow.removeCategory(editing.index);
    form.clearErrors('categories');
    setEditing(null);
  };

  if (editing) {
    return (
      <CategoryEditor
        flow={flow}
        index={editing.index}
        view={editing.view}
        onView={(view) => {
          setEditing({ ...editing, view });
        }}
        onDiscard={discard}
        onSaved={() => {
          setEditing(null);
        }}
      />
    );
  }
  return (
    <>
      <CategorySummaryList
        categories={saved}
        error={listError}
        onAdd={() => {
          open(flow.addCategory(), null);
        }}
        onEdit={(index) => {
          const current = saved[index];
          if (current) open(index, { ...current });
        }}
        onRemove={flow.removeCategory}
      />
      <SectionFooter
        onCancel={onBack}
        cancelLabel={ES_AR.common.back}
        submitLabel={ES_AR.common.next}
        onSubmit={() => {
          void form.trigger('categories').then((valid) => {
            if (valid) onNext();
          });
        }}
      />
    </>
  );
}
