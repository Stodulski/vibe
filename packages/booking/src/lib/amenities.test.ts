import { describe, expect, it } from 'vitest';
import { AMENITIES, orderedAmenities } from './amenities';

describe('amenities', () => {
  it('labels the seven new amenities and the two relabelled ones', () => {
    const labels = Object.fromEntries(AMENITIES.map((a) => [a.value, a.label]));
    expect(labels).toMatchObject({
      birthdays: 'Cumpleaños',
      grill: 'Parrilla',
      quincho: 'Quincho',
      medical_aid: 'Ayuda médica',
      sports_school: 'Escuelita deportiva',
      kiosk: 'Kiosco o buffet',
      kids_area: 'Juegos para chicos',
      racket_rental: 'Alquiler de equipo',
      pro_shop: 'Venta de pelotas y accesorios',
    });
  });

  it('lists a selected new amenity in the canonical order', () => {
    expect(orderedAmenities(['kids_area', 'parking']).map((a) => a.value)).toEqual(['parking', 'kids_area']);
  });
});
