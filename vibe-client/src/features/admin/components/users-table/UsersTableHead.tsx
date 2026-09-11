import { TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function UsersTableHead() {
  return (
    <TableHeader>
      <TableRow>
        <TableHead>{t.admin.users.name}</TableHead>
        <TableHead>{t.admin.users.email}</TableHead>
        <TableHead>{t.admin.users.role}</TableHead>
        <TableHead className="text-center">{t.admin.users.complexes}</TableHead>
        <TableHead className="text-center">{t.admin.users.status}</TableHead>
        <TableHead>{t.admin.users.created}</TableHead>
      </TableRow>
    </TableHeader>
  );
}
