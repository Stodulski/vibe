// Public API of the admin feature.
// Only export what other features or pages actually consume from outside
// this folder — see 06-auth-shared-tooling.md A3.

export { ComplexesTable } from './components/ComplexesTable';
export { AdminStatsCards } from './components/AdminStatsCards';
export { UserDetailPanel } from './components/UserDetailPanel';
export { ComplexDetailPanel } from './components/ComplexDetailPanel';
export { UsersTable } from './components/UsersTable';

export { useAdminStats } from './hooks/useAdminStats';
export { useAdminUserDetail } from './hooks/useAdminUserDetail';
export { useAdminComplexDetail } from './hooks/useAdminComplexDetail';
