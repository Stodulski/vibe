import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { categoryField } from '../../schemas/tournamentForm.schema';
import { CategoryBasicsFields, CategoryPlayFields } from './CategoryFields';
import { categoryViewFields, playViewFields } from './stepFields';
import type { CreateTournamentForm } from './useCreateTournamentForm';

const t = ES_AR.tournaments.createSteps;

export type CategoryView = 'category' | 'play';

interface Props {
  flow: CreateTournamentForm;
  index: number;
  view: CategoryView;
  onView: (view: CategoryView) => void;
  /** Leaves the editor without keeping what was typed in the first view. */
  onDiscard: () => void;
  onSaved: () => void;
}

/**
 * One category as two short views inside step 2: "Categoría" (name, format, capacity, price,
 * court fee) and "Cómo se juega" (zones, match format, games, duration). Each view validates its
 * own fields before moving on.
 */
export function CategoryEditor({ flow, index, view, onView, onDiscard, onSaved }: Props) {
  const { form } = flow;

  const goPlay = async () => {
    if (await form.trigger(categoryViewFields(index))) onView('play');
  };
  const save = async () => {
    const format = form.getValues(categoryField(index, 'format'));
    if (await form.trigger(playViewFields(index, format))) onSaved();
  };

  if (view === 'category') {
    return (
      <>
        <div className="space-y-4">
          <h3 className="text-text-primary text-sm font-semibold">{t.categoryView}</h3>
          <CategoryBasicsFields form={form} index={index} />
        </div>
        <SectionFooter
          onCancel={onDiscard}
          cancelLabel={ES_AR.common.back}
          submitLabel={ES_AR.common.next}
          onSubmit={() => void goPlay()}
        />
      </>
    );
  }
  return (
    <>
      <div className="space-y-4">
        <h3 className="text-text-primary text-sm font-semibold">{t.playView}</h3>
        <CategoryPlayFields form={form} index={index} />
      </div>
      <SectionFooter
        onCancel={() => {
          onView('category');
        }}
        cancelLabel={ES_AR.common.back}
        submitLabel={t.saveCategory}
        onSubmit={() => void save()}
      />
    </>
  );
}
