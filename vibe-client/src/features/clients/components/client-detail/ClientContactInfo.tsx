import type { Client } from '@/shared/types/api.types';

interface ClientContactInfoProps {
  client: Client;
}

export function ClientContactInfo({ client }: ClientContactInfoProps) {
  if (!client.email) return null;

  return <p className="text-sm text-text-secondary">{client.email}</p>;
}
