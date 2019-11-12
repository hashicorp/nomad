import { module, test } from 'qunit';
import parseDuration from 'nomad-ui/utils/parse-duration';

const testCases = [
  {
    name: 'Only seconds',
    in: '5s',
    out: 5 * 1000 * 1000000,
  },
  {
    name: 'Only minutes',
    in: '30m',
    out: 30 * 60 * 1000 * 1000000,
  },
  {
    name: 'Only hours',
    in: '8h',
    out: 8 * 60 * 60 * 1000 * 1000000,
  },
  {
    name: 'Only days',
    in: '2d',
    out: 2 * 24 * 60 * 60 * 1000 * 1000000,
  },
  {
    name: 'Composite',
    in: '1d8h15m30s',
    out: (((1 * 24 + 8) * 60 + 15) * 60 + 30) * 1000 * 1000000,
  },
  {
    name: 'Zeroes',
    in: '0d0h0m0s',
    out: 0,
  },
  {
    name: 'Improper durations',
    in: '90m',
    out: 90 * 60 * 1000 * 1000000,
  },
  {
    name: 'Already parsed',
    in: 1000000,
    out: 1000000,
  },
];

const errorCases = [
  {
    name: 'Empty string',
    in: '',
    error: /Durations must start with a numeric quantity/,
  },
  {
    name: 'No quantity',
    in: 'h',
    error: /Durations must start with a numeric quantity/,
  },
  {
    name: 'Unallowed unit',
    in: '15M',
    error: /Unallowed duration unit "M"/,
  },
  {
    name: 'Float quantities',
    in: '1.5m',
    error: /Unallowed duration unit "\."/,
  },
  {
    name: 'Repeated unit tokens',
    in: '30hm',
    error: /Cannot follow a non-numeric token with a non-numeric token/,
  },
];

module('Unit | Util | parseDuration', function() {
  testCases.forEach(testCase => {
    test(testCase.name, function(assert) {
      assert.equal(parseDuration(testCase.in), testCase.out, `'${testCase.in}' => ${testCase.out}`);
    });
  });

  errorCases.forEach(testCase => {
    test(`Error Case: ${testCase.name}`, function(assert) {
      assert.throws(
        () => {
          parseDuration(testCase.in);
        },
        testCase.error,
        `'${testCase.in}' throws ${testCase.error}`
      );
    });
  });
});
