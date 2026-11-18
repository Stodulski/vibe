import { test } from 'node:test';
import assert from 'node:assert/strict';
import { sharedRead } from '../src/lib/kit/shared-read.ts';

test('requests for the same key at once share one read', async () => {
  let calls = 0;
  const read = sharedRead(async (key: string) => {
    calls += 1;
    return `facts of ${key}`;
  }, 60_000);
  const answers = await Promise.all([read('club'), read('club'), read('club'), read('club')]);
  assert.deepEqual(answers, Array(4).fill('facts of club'));
  assert.equal(calls, 1);
});

test('another key, or the same key after the time is up, reads again', async () => {
  let clock = 0;
  let calls = 0;
  const read = sharedRead(
    async (key: string) => {
      calls += 1;
      return key;
    },
    1_000,
    () => clock,
  );
  await read('a');
  await read('b');
  clock = 1_001;
  await read('a');
  assert.equal(calls, 3);
});

test('a failed read is not kept: the next request asks again', async () => {
  let calls = 0;
  const read = sharedRead(async () => {
    calls += 1;
    if (calls === 1) throw new Error('timeout');
    return 'ok';
  }, 60_000);
  await assert.rejects(read('club'));
  assert.equal(await read('club'), 'ok');
  assert.equal(calls, 2);
});
