/**
 * The layout of the Instagram stories: the kit's story frame (photo, tint, glow, the margins that
 * keep Instagram's bars off the words) with the free hours or the freed slot in the middle, and
 * the complex's address at the bottom. No QR: a story is seen on the phone that would scan it.
 */
import type { CSSProperties, ReactElement, ReactNode } from 'react';
import { BG, FONT_FAMILY, GREEN, MUTED, TEXT, box, glow } from '../og/render.ts';
import { kitLayout } from '../kit/kit-image.ts';
import { GLASS, SHADOW, backdrop, brandMark, type KitArt } from '../kit/kit-tree.ts';
import type { SportHours, StoryText } from './story-text.ts';

export const STORY_WIDTH = 1080;
export const STORY_HEIGHT = 1920;

const LAYOUT = kitLayout('historia');
const CENTERED: CSSProperties = { flexDirection: 'column', alignItems: 'center' };

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

function titleBlock(text: StoryText): ReactElement {
  return box(
    { ...CENTERED, width: '100%', gap: 24 },
    box(
      {
        display: '-webkit-box',
        width: '100%',
        fontSize: LAYOUT.nameSize,
        fontWeight: 700,
        lineHeight: 1.05,
        letterSpacing: -2,
        textAlign: 'center',
        overflow: 'hidden',
        WebkitLineClamp: 2,
        WebkitBoxOrient: 'vertical',
        textOverflow: 'ellipsis',
        textShadow: SHADOW,
      },
      text.name,
    ),
    box(
      { fontSize: 72, fontWeight: 700, color: GREEN, textAlign: 'center', letterSpacing: -1, textShadow: SHADOW },
      text.headline,
    ),
  );
}

/** One free hour as a pill. */
function hourChip(hour: string): ReactElement {
  return box(
    {
      padding: '14px 30px',
      borderRadius: 999,
      backgroundColor: 'rgba(255,255,255,0.1)',
      border: `2px solid rgba(29,185,84,0.7)`,
      fontSize: 52,
      fontWeight: 700,
    },
    hour,
  );
}

function sportBlock(section: SportHours): ReactElement {
  return box(
    { ...CENTERED, gap: 24, width: '100%' },
    box({ fontSize: 44, fontWeight: 700, color: MUTED }, section.sport),
    box({ flexWrap: 'wrap', justifyContent: 'center', gap: 20, width: '100%' }, ...section.hours.map(hourChip)),
    ...(section.more > 0
      ? [box({ fontSize: 38, color: MUTED }, section.more === 1 ? 'y 1 horario más' : `y ${section.more} horarios más`)]
      : []),
  );
}

function freeCard(text: StoryText): ReactElement {
  return box(
    { ...GLASS, ...CENTERED, gap: 48, width: '100%', padding: '48px 40px' },
    ...text.sections.map(sportBlock),
  );
}

function freedCard(text: StoryText): ReactElement {
  const slot = text.slot;
  if (!slot) return box({});
  return box(
    { ...GLASS, ...CENTERED, gap: 16, width: '100%', padding: '56px 40px' },
    box({ fontSize: 52, color: MUTED }, capitalize(slot.day)),
    box({ fontSize: 220, fontWeight: 700, lineHeight: 1, letterSpacing: -6, color: TEXT }, slot.time),
    box(
      {
        display: '-webkit-box',
        maxWidth: '100%',
        fontSize: 52,
        fontWeight: 700,
        textAlign: 'center',
        overflow: 'hidden',
        WebkitLineClamp: 2,
        WebkitBoxOrient: 'vertical',
        textOverflow: 'ellipsis',
      },
      slot.sport ? `${slot.court} · ${slot.sport}` : slot.court,
    ),
  );
}

/** The address: the only way in a story offers, typed or tapped from the profile. */
function addressBlock(text: StoryText): ReactElement {
  return box(
    { ...CENTERED, gap: 12 },
    box({ fontSize: 44, color: MUTED, textShadow: SHADOW }, 'Reservá en'),
    box({ fontSize: 52, fontWeight: 700, textAlign: 'center', textShadow: SHADOW }, text.link),
  );
}

export function storyTree(text: StoryText, art: KitArt): ReactElement {
  const middle = text.slot ? freedCard(text) : freeCard(text);
  // The logo keeps its size; the column shrinks the card instead.
  const blocks: ReactNode[] = [titleBlock(text), middle, addressBlock(text), box({ flexShrink: 0 }, brandMark(art.logo))];
  return box(
    {
      position: 'relative',
      width: STORY_WIDTH,
      height: STORY_HEIGHT,
      flexDirection: 'column',
      overflow: 'hidden',
      backgroundColor: BG,
      color: TEXT,
      fontFamily: FONT_FAMILY,
    },
    ...backdrop(art.photo, LAYOUT),
    glow({ top: -200, right: -260 }, 900),
    box(
      {
        position: 'absolute',
        top: LAYOUT.top,
        right: LAYOUT.side,
        bottom: LAYOUT.bottom,
        left: LAYOUT.side,
        ...CENTERED,
        justifyContent: 'space-between',
        gap: LAYOUT.gap,
      },
      ...blocks,
    ),
  );
}
