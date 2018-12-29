import { module, test } from 'ember-qunit';
import { formatBytes } from 'nomad-ui/helpers/format-bytes';

module('format-bytes', 'Unit | Helper | format-bytes');

test('formats x < 1024 as bytes', function(assert) {
  assert.equal(formatBytes([0]), '0 Bytes');
  assert.equal(formatBytes([100]), '100 Bytes');
  assert.equal(formatBytes([1023]), '1023 Bytes');
});

test('formats 1024 <= x < 1024 * 1024 as KiB', function(assert) {
  assert.equal(formatBytes([1024]), '1 KiB');
  assert.equal(formatBytes([125952]), '123 KiB');
  assert.equal(formatBytes([1024 * 1024 - 1]), '1023 KiB');
});

test('formats 1024 * 1024 <= x < 1024 * 1024 * 1024 as MiB', function(assert) {
  assert.equal(formatBytes([1024 * 1024]), '1 MiB');
  assert.equal(formatBytes([128974848]), '123 MiB');
});

test('formats x > 1024 * 1024 * 1024 as MiB, since it is the highest allowed unit', function(
  assert
) {
  assert.equal(formatBytes([1024 * 1024 * 1024]), '1024 MiB');
  assert.equal(formatBytes([1024 * 1024 * 1024 * 4]), '4096 MiB');
});
