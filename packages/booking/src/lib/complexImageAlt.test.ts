import { describe, expect, it } from 'vitest';
import { complexCoverAlt, complexLogoAlt } from './complexImageAlt';

describe('complexCoverAlt', () => {
  it('names the club, its sports and its city', () => {
    expect(complexCoverAlt({ name: 'Club Padel Norte', city: 'Buenos Aires', sports: ['Pádel', 'Fútbol'] })).toBe(
      'Club Padel Norte, canchas de pádel y fútbol en Buenos Aires',
    );
  });

  it('leaves the city out when the complex has none', () => {
    expect(complexCoverAlt({ name: 'Club Padel Norte', city: '', sports: ['Pádel', 'Fútbol'] })).toBe(
      'Club Padel Norte, canchas de pádel y fútbol',
    );
  });

  it('says only the club and its city when no sport is known', () => {
    expect(complexCoverAlt({ name: 'Club Padel Norte', city: 'Rosario', sports: [] })).toBe(
      'Club Padel Norte en Rosario',
    );
  });

  it('is the club name alone when neither sport nor city is known', () => {
    expect(complexCoverAlt({ name: 'Club Padel Norte', city: ' ', sports: [] })).toBe('Club Padel Norte');
  });

  it('ignores blank and repeated sports and names at most three', () => {
    expect(
      complexCoverAlt({
        name: 'Club',
        city: 'Rosario',
        sports: ['Pádel', '', 'Pádel', 'Tenis', 'Fútbol', 'Vóley'],
      }),
    ).toBe('Club, canchas de pádel, tenis y fútbol en Rosario');
  });

  it('drops sports before it passes 125 characters', () => {
    const name =
      'Complejo Deportivo Muy Largo De Nombre Extenso Para Probar El Limite Del Texto Alternativo Central Sur Oeste';
    const alt = complexCoverAlt({ name, city: 'Rosario', sports: ['Pádel', 'Tenis', 'Fútbol'] });

    expect(alt.length).toBeLessThanOrEqual(125);
    expect(alt).toBe(`${name} en Rosario`);
  });

  it('keeps the sports when the alt fits within 125 characters', () => {
    const alt = complexCoverAlt({ name: 'Club Norte', city: 'Rosario', sports: ['Pádel', 'Tenis', 'Fútbol'] });

    expect(alt).toBe('Club Norte, canchas de pádel, tenis y fútbol en Rosario');
  });
});

describe('complexLogoAlt', () => {
  it('names the club as the logo', () => {
    expect(complexLogoAlt('Club Padel Norte')).toBe('Logo de Club Padel Norte');
  });
});
