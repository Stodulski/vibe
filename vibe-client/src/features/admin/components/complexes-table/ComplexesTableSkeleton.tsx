import { DesktopTableShell } from '@/shared/components/common/DesktopTableShell';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Table, TableBody, TableCell, TableRow } from '@/shared/components/ui/table';
import { ComplexesTableHead } from './ComplexesTableHead';

export function ComplexesTableSkeleton() {
  return (
    <>
      <div className="flex flex-col gap-2 md:hidden">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-[76px] rounded-xl" />
        ))}
      </div>
      <DesktopTableShell>
        <Table>
          <ComplexesTableHead />
          <TableBody>
            {Array.from({ length: 5 }).map((_, i) => (
              <TableRow key={i}>
                <TableCell>
                  <Skeleton className="h-4 w-32" />
                </TableCell>
                <TableCell>
                  <Skeleton className="h-4 w-24" />
                </TableCell>
                <TableCell>
                  <Skeleton className="h-4 w-28" />
                </TableCell>
                <TableCell className="text-center">
                  <Skeleton className="h-4 w-8 mx-auto" />
                </TableCell>
                <TableCell className="text-center">
                  <Skeleton className="h-4 w-8 mx-auto" />
                </TableCell>
                <TableCell className="text-center">
                  <Skeleton className="h-5 w-16 mx-auto" />
                </TableCell>
                <TableCell>
                  <Skeleton className="h-4 w-24" />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </DesktopTableShell>
    </>
  );
}
