import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { ClientGridSkeleton } from '../components/ClientGridSkeleton';
import { ClientsSearchBarSkeleton } from './clients/ClientsSearchBarSkeleton';
import { CLIENTS_BODY_CLASS } from './clients/clientsSearchBarLayout';

/** What `ClientsPage` draws while its list loads, chrome included: the mobile title, the search bar, then the list. */
export function ClientsPageSkeleton() {
  return (
    <div>
      <PageHeaderSkeleton />
      <div className={CLIENTS_BODY_CLASS}>
        <ClientsSearchBarSkeleton />
        <ClientGridSkeleton />
      </div>
    </div>
  );
}
