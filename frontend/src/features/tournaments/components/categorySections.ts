import { ES_AR } from '@/shared/i18n/es_AR';
import type { TournamentOwnerCategory } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

/** The sections of one category, chosen from the owner's section select. */
export type Section = 'pairs' | 'groups' | 'bracket' | 'matches';

/** The sections a category offers: its zones only when it is played in zones. */
export function sectionItems(category: TournamentOwnerCategory): { value: Section; label: string }[] {
  const s = t.detail.sections;
  return [
    { value: 'pairs', label: s.pairs },
    ...(category.format === 'groups' ? [{ value: 'groups' as const, label: s.groups }] : []),
    { value: 'bracket', label: s.bracket },
    { value: 'matches', label: s.matches },
  ];
}

/** A category opens on its matches once drawn, and on its pairs before the draw. */
export function openingSection(category: TournamentOwnerCategory): Section {
  return category.draw_generated ? 'matches' : 'pairs';
}
