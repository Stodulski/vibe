import type { Client } from '@/shared/types/api.types';

interface ClientContactInfoProps {
  client: Client;
}

export function ClientContactInfo({ client }: ClientContactInfoProps) {
  if (!client.email) return null;

  return <p className="text-text-secondary text-sm">{client.email}</p>;
}
