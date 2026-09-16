import { ES_AR } from '@/shared/i18n/es_AR';
import type { Sport, CourtType } from '@/shared/types/api.types';

const t = ES_AR;

export const SPORTS: { value: Sport; label: string }[] = [
  { value: 'padel', label: t.courts.sportTypes.padel },
  { value: 'tennis', label: t.courts.sportTypes.tennis },
  { value: 'soccer', label: t.courts.sportTypes.soccer },
  { value: 'basketball', label: t.courts.sportTypes.basketball },
  { value: 'volleyball', label: t.courts.sportTypes.volleyball },
  { value: 'hockey', label: t.courts.sportTypes.hockey },
  { value: 'pickleball', label: t.courts.sportTypes.pickleball },
];

export const COURT_TYPES: { value: CourtType; label: string }[] = [
  { value: 'indoor', label: t.courts.courtTypes.indoor },
  { value: 'outdoor', label: t.courts.courtTypes.outdoor },
  { value: 'semi_covered', label: t.courts.courtTypes.semi_covered },
];
