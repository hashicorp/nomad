/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import * as units from 'nomad-ui/utils/units';

function table(fn, cases) {
  cases.forEach((testCase) => {
    test(testCase.name || testCase.out, function (assert) {
      assert.deepEqual(fn.apply(null, testCase.in), testCase.out);
    });
  });
}

module('Unit | Util | units#formatBytes', function () {
  table.call(this, units.formatBytes, [
    { in: [null], out: '0 Bytes', name: 'formats null as 0 bytes' },
    { in: [undefined], out: '0 Bytes', name: 'formats undefined as 0 bytes' },
    { in: [0], out: '0 Bytes', name: 'formats x < 1024 as bytes' },
    { in: [100], out: '100 Bytes' },
    { in: [1023], out: '1,023 Bytes' },
    { in: [1024], out: '1 KiB', name: 'formats 1024 <= x < 1024^2 as KiB' },
    { in: [1024 ** 2 - 1024 * 0.01], out: '1,023.99 KiB' },
    {
      in: [1024 ** 2],
      out: '1 MiB',
      name: 'formats 1024^2 <= x < 1024^3 as MiB',
    },
    { in: [1024 ** 2 * 1.016], out: '1.02 MiB' },
    {
      in: [1024 ** 3],
      out: '1 GiB',
      name: 'formats 1024^3 <= x < 1024^4 as GiB',
    },
    { in: [1024 ** 3 * 512.5], out: '512.5 GiB' },
    {
      in: [1024 ** 4],
      out: '1 TiB',
      name: 'formats 1024^4 <= x < 1024^5 as TiB',
    },
    { in: [1024 ** 4 * 2.1234], out: '2.12 TiB' },
    { in: [1024 ** 5], out: '1 PiB', name: 'formats x > 1024^5 as PiB' },
    { in: [1024 ** 5 * 4000], out: '4,000 PiB' },
    {
      in: [1024 ** 2, 'MiB'],
      out: '1 TiB',
      name: 'accepts a starting unit size as an optional argument',
    },
    {
      in: [1024 ** 2 * -1],
      out: '-1 MiB',
      name: 'negative values are still reduced',
    },
  ]);
});

module('Unit | Util | units#formatScheduledBytes', function () {
  table.call(this, units.formatScheduledBytes, [
    { in: [null], out: '0 Bytes', name: 'formats null as 0 bytes' },
    { in: [undefined], out: '0 Bytes', name: 'formats undefined as 0 bytes' },
    { in: [0], out: '0 Bytes', name: 'formats x < 1024 as bytes' },
    {
      in: [1024 ** 3],
      out: '1,024 MiB',
      name: 'max unit is MiB, just like Nomad expects in job specs',
    },
    {
      in: [1024 ** 3 + 1024 ** 2 * 0.6],
      out: '1,025 MiB',
      name: 'MiBs are rounded to whole numbers',
    },
    {
      in: [2000, 'MiB'],
      out: '2,000 MiB',
      name: 'accepts a starting unit size as an optional argument',
    },
    {
      in: [1024 ** 3 * -1],
      out: '-1,024 MiB',
      name: 'negative values are still reduced',
    },
  ]);
});

module('Unit | Util | units#formatHertz', function () {
  table.call(this, units.formatHertz, [
    { in: [null], out: '0 Hz', name: 'formats null as 0 Hz' },
    { in: [undefined], out: '0 Hz', name: 'formats undefined as 0 Hz' },
    { in: [0], out: '0 Hz', name: 'formats x < 1000 as Hz' },
    { in: [999], out: '999 Hz' },
    { in: [1000], out: '1 KHz', name: 'formats 1000 <= x < 1000^2 as KHz' },
    { in: [1000 ** 2 - 10], out: '999.99 KHz' },
    {
      in: [1000 ** 2],
      out: '1 MHz',
      name: 'formats 1000^2 <= x < 1000^3 as MHz',
    },
    { in: [1000 ** 2 * 5.234], out: '5.23 MHz' },
    {
      in: [1000 ** 3],
      out: '1 GHz',
      name: 'formats 1000^3 <= x < 1000^4 as GHz',
    },
    { in: [1000 ** 3 * 500.238], out: '500.24 GHz' },
    {
      in: [1000 ** 4],
      out: '1 THz',
      name: 'formats 1000^4 <= x < 1000^5 as THz',
    },
    { in: [1000 ** 4 * 12], out: '12 THz' },
    { in: [1000 ** 5], out: '1 PHz', name: 'formats x > 1000^5 as PHz' },
    { in: [1000 ** 5 * 34567.89], out: '34,567.89 PHz' },
    {
      in: [2000, 'KHz'],
      out: '2 MHz',
      name: 'accepts a starting unit size as an optional argument',
    },
    {
      in: [1000 ** 3 * -1],
      out: '-1 GHz',
      name: 'negative values are still reduced',
    },
  ]);
});

module('Unit | Util | units#formatScheduledHertz', function () {
  table.call(this, units.formatScheduledHertz, [
    { in: [null], out: '0 Hz', name: 'formats null as 0 Hz' },
    { in: [undefined], out: '0 Hz', name: 'formats undefined as 0 Hz' },
    { in: [0], out: '0 Hz', name: 'formats x < 1024 as Hz' },
    {
      in: [1000 ** 3],
      out: '1,000 MHz',
      name: 'max unit is MHz, just like Nomad expects in job specs',
    },
    {
      in: [1000 ** 3 + 1000 ** 2 * 0.6],
      out: '1,001 MHz',
      name: 'MHz are rounded to whole numbers',
    },
    {
      in: [2000, 'MHz'],
      out: '2,000 MHz',
      name: 'accepts a starting unit size as an optional argument',
    },
    {
      in: [1000 ** 3 * -1],
      out: '-1,000 MHz',
      name: 'negative values are still reduced',
    },
  ]);
});

module('Unit | Util | units#reduceBytes', function () {
  table.call(this, units.reduceBytes, [
    {
      in: [],
      out: [0, 'Bytes'],
      name: 'No args behavior results in valid output',
    },
    { in: [1024 ** 6], out: [1024, 'PiB'], name: 'Max default unit is PiB' },
    {
      in: [1024 ** 6 * 1.12345],
      out: [1024 * 1.12345, 'PiB'],
      name: 'Returned numbers are not formatted',
    },
    {
      in: [1024 ** 2 * 2, 'KiB'],
      out: [2048, 'KiB'],
      name: 'Reduction ends when the specified max unit is reached',
    },
    {
      in: [1024 ** 2, 'MiB', 'KiB'],
      out: [1024, 'MiB'],
      name: 'accepts a starting unit size as an optional argument',
    },
    {
      in: [1024 ** 3 * -1],
      out: [-1, 'GiB'],
      name: 'negative values are still reduced',
    },
  ]);
});

module('Unit | Util | units#reduceHertz', function () {
  table.call(this, units.reduceHertz, [
    {
      in: [],
      out: [0, 'Hz'],
      name: 'No args behavior results in valid output',
    },
    { in: [1000 ** 6], out: [1000, 'PHz'], name: 'Max default unit is PHz' },
    {
      in: [1000 ** 6 * 1.12345],
      out: [1000 * 1.12345, 'PHz'],
      name: 'Returned numbers are not formatted',
    },
    {
      in: [1000 ** 4 * 2, 'GHz'],
      out: [2000, 'GHz'],
      name: 'Reduction ends when the specified max unit is reached',
    },
    {
      in: [1000 * 2, 'THz', 'MHz'],
      out: [2, 'GHz'],
      name: 'accepts a starting unit size as an optional argument',
    },
    {
      in: [1000 ** 3 * -1],
      out: [-1, 'GHz'],
      name: 'negative values are still reduced',
    },
  ]);
});
