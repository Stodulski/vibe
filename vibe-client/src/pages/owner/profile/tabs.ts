import { UserRound, Shield } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export const TABS = [
  {
    value: 'personal',
    label: t.profile.personal,
    icon: UserRound,
  },
  {
    value: 'security',
    label: t.profile.security,
    icon: Shield,
  },
] as const;

export type TabValue = (typeof TABS)[number]['value'];
