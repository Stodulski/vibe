// Public API of the clients feature.
// Only export what other features or pages actually consume from outside
// this folder — see 06-auth-shared-tooling.md A3 and 02-bookings-clients.md M4.

export { ClientDetail } from './components/ClientDetail';
export { BlockClientModal } from './components/BlockClientModal';
export { ClientGrid } from './components/ClientGrid';
export { ClientDrawerModals } from './components/ClientDrawerModals';

export { useClients } from './hooks/useClients';
export { useClientActions } from './hooks/useClientActions';
export { useClient } from './hooks/useClient';
