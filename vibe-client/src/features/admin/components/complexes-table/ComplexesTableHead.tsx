import { TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function ComplexesTableHead() {
  return (
    <TableHeader>
      <TableRow>
        <TableHead>{t.admin.complexes.name}</TableHead>
        <TableHead>{t.admin.complexes.city}</TableHead>
        <TableHead>{t.admin.complexes.owner}</TableHead>
        <TableHead className="text-center">{t.admin.complexes.courts}</TableHead>
        <TableHead className="text-center">{t.admin.complexes.mpConnected}</TableHead>
        <TableHead className="text-center">{t.admin.complexes.status}</TableHead>
        <TableHead>{t.admin.complexes.created}</TableHead>
      </TableRow>
    </TableHeader>
  );
}
