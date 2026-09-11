import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export const roleOptions = [
  { value: '_all', label: t.admin.users.allRoles },
  { value: 'owner', label: t.admin.roles.owner },
  { value: 'superadmin', label: t.admin.roles.superadmin },
];

export function getRoleLabel(role: string): string {
  return (t.admin.roles as Record<string, string>)[role] ?? role;
}
