import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { useForm } from 'react-hook-form';
import { EventCourtsField } from './EventCourtsField';
import type { CourtWithPrices } from '@/shared/types/api.types';
import type { EventFormValues } from '../../schemas/eventForm.schema';

function court(id: string, name: string, facts: Partial<CourtWithPrices>): CourtWithPrices {
  return {
    id,
    complex_id: 'c1',
    name,
    sport: 'padel',
    court_type: 'outdoor',
    is_active: true,
    has_lighting: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    prices: [],
    ...facts,
  };
}

function Harness({ courts }: { courts: CourtWithPrices[] }) {
  const form = useForm<EventFormValues>({
    defaultValues: { allCourts: false, courtIds: [] } as Partial<EventFormValues> as EventFormValues,
  });
  return <EventCourtsField form={form} courts={courts} />;
}

describe('EventCourtsField', () => {
  it('describes each court by its roof, surface and lighting, so the owner can tell them apart', () => {
    render(
      <Harness
        courts={[
          court('ct1', 'Cancha 1', { court_type: 'indoor', surface: 'clay', has_lighting: true }),
          court('ct2', 'Cancha 2', { court_type: 'outdoor' }),
        ]}
      />,
    );

    expect(screen.getByRole('checkbox', { name: 'Cancha 1' })).toHaveAccessibleDescription(
      'Techada · Polvo de ladrillo · Con iluminación',
    );
    expect(screen.getByRole('checkbox', { name: 'Cancha 2' })).toHaveAccessibleDescription('Descubierta');
  });
});
