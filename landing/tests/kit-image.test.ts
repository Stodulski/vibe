/**
 * The kit's picture names, sizes and layout: which piece and format a path names, the size each
 * format draws, and the space the stories keep free of key text. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { kitFileName, kitLayout, parseKitImage, wantsDownload } from '../src/lib/kit/kit-image.ts';

test('the two pieces in both formats parse to their piece and format', () => {
  assert.deepEqual(parseKitImage('reservar-historia'), { piece: 'reservar', format: 'historia' });
  assert.deepEqual(parseKitImage('reservar-post'), { piece: 'reservar', format: 'post' });
  assert.deepEqual(parseKitImage('como-reservar-historia'), { piece: 'como-reservar', format: 'historia' });
  assert.deepEqual(parseKitImage('como-reservar-post'), { piece: 'como-reservar', format: 'post' });
});

test('any other piece or format is not a kit picture', () => {
  const names = [
    '',
    'reservar',
    'reservar-',
    'historia',
    'post',
    'reservar-cuadrado',
    'canchas-post',
    'como-reservar-Post',
    'reservar-historia-post',
    'otra-historia',
    'reservar-historia ',
  ];
  for (const name of names) assert.equal(parseKitImage(name), null, name);
});

test('the stories are 1080x1920 and the posts 1080x1440', () => {
  assert.deepEqual([kitLayout('historia').width, kitLayout('historia').height], [1080, 1920]);
  assert.deepEqual([kitLayout('post').width, kitLayout('post').height], [1080, 1440]);
});

test('stories keep their top 220px and bottom 260px free of key text', () => {
  const layout = kitLayout('historia');
  assert.equal(layout.top, 220);
  assert.equal(layout.bottom, 260);
});

test('the posts keep a 64px margin on every side', () => {
  const layout = kitLayout('post');
  assert.equal(layout.top, 64);
  assert.equal(layout.bottom, 64);
  assert.equal(layout.side, 64);
});

test('every format keeps the 64px side margin and key text clear of the top and bottom it reserves', () => {
  for (const format of ['historia', 'post'] as const) {
    const layout = kitLayout(format);
    assert.equal(layout.side, 64, format);
    assert.ok(layout.top >= 64 && layout.bottom >= 64, format);
  }
});

test('the download name carries the slug, the piece and the format', () => {
  assert.equal(
    kitFileName('la-zona-f5', { piece: 'como-reservar', format: 'post' }),
    'vibe-la-zona-f5-como-reservar-post.png',
  );
});

test('only ?descargar=1 asks for the download', () => {
  assert.equal(wantsDownload(new URLSearchParams('descargar=1')), true);
  assert.equal(wantsDownload(new URLSearchParams('')), false);
  assert.equal(wantsDownload(new URLSearchParams('descargar=0')), false);
  assert.equal(wantsDownload(new URLSearchParams('descargar=true')), false);
});
