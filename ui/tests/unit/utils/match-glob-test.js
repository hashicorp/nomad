/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import matchGlob from 'nomad-ui/utils/match-glob';
import { module, test } from 'qunit';

module('Unit | Utility | match-glob', function () {
  module('#_doesMatchPattern', function () {
    const edgeCaseTest = 'this is a ϗѾ test';

    module('base cases', function () {
      test('it handles an empty pattern', function (assert) {
        // arrange
        const pattern = '';
        const emptyPath = '';
        const nonEmptyPath = 'a';

        // act
        const matchingResult = matchGlob(pattern, emptyPath);
        const nonMatchingResult = matchGlob(pattern, nonEmptyPath);

        // assert
        assert.ok(matchingResult, 'Empty pattern should match empty path');
        assert.notOk(
          nonMatchingResult,
          'Empty pattern should not match non-empty path'
        );
      });

      test('it handles an empty path', function (assert) {
        // arrange
        const emptyPath = '';
        const emptyPattern = '';
        const nonEmptyPattern = 'a';

        // act
        const matchingResult = matchGlob(emptyPattern, emptyPath);
        const nonMatchingResult = matchGlob(nonEmptyPattern, emptyPath);

        // assert
        assert.ok(matchingResult, 'Empty path should match empty pattern');
        assert.notOk(
          nonMatchingResult,
          'Empty path should not match non-empty pattern'
        );
      });

      test('it handles a pattern without a glob', function (assert) {
        // arrange
        const path = '/foo';
        const matchingPattern = '/foo';
        const nonMatchingPattern = '/bar';

        // act
        const matchingResult = matchGlob(matchingPattern, path);
        const nonMatchingResult = matchGlob(nonMatchingPattern, path);

        // assert
        assert.ok(matchingResult, 'Matches path correctly.');
        assert.notOk(nonMatchingResult, 'Does not match non-matching path.');
      });

      test('it handles a pattern that is a lone glob', function (assert) {
        // arrange
        const path = '/foo';
        const glob = '*';

        // act
        const matchingResult = matchGlob(glob, path);

        // assert
        assert.ok(matchingResult, 'Matches glob.');
      });

      test('it matches on leading glob', function (assert) {
        // arrange
        const pattern = '*bar';
        const matchingPath = 'footbar';
        const nonMatchingPath = 'rockthecasba';

        // act
        const matchingResult = matchGlob(pattern, matchingPath);
        const nonMatchingResult = matchGlob(pattern, nonMatchingPath);

        // assert
        assert.ok(
          matchingResult,
          'Correctly matches when leading glob and matching path.'
        );
        assert.notOk(
          nonMatchingResult,
          'Does not match when leading glob and non-matching path.'
        );
      });

      test('it matches on trailing glob', function (assert) {
        // arrange
        const pattern = 'foo*';
        const matchingPath = 'footbar';
        const nonMatchingPath = 'bar';

        // act
        const matchingResult = matchGlob(pattern, matchingPath);
        const nonMatchingResult = matchGlob(pattern, nonMatchingPath);

        // assert
        assert.ok(matchingResult, 'Correctly matches on trailing glob.');
        assert.notOk(
          nonMatchingResult,
          'Does not match on trailing glob if pattern does not match.'
        );
      });

      test('it matches when glob is in middle', function (assert) {
        // arrange
        const pattern = 'foo*bar';
        const matchingPath = 'footbar';
        const nonMatchingPath = 'footba';

        // act
        const matchingResult = matchGlob(pattern, matchingPath);
        const nonMatchingResult = matchGlob(pattern, nonMatchingPath);

        // assert
        assert.ok(
          matchingResult,
          'Correctly matches on glob in middle of path.'
        );
        assert.notOk(
          nonMatchingResult,
          'Does not match on glob in middle of path if not full pattern match.'
        );
      });
    });

    module('matching edge cases', function () {
      test('it matches when string is between globs', function (assert) {
        // arrange
        const pattern = '*is *';

        // act
        const result = matchGlob(pattern, edgeCaseTest);

        // assert
        assert.ok(result);
      });

      test('it handles many non-consective globs', function (assert) {
        // arrange
        const pattern = '*is*a*';

        // act
        const result = matchGlob(pattern, edgeCaseTest);

        // assert
        assert.ok(result);
      });

      test('it handles double globs', function (assert) {
        // arrange
        const pattern = '**test**';

        // act
        const result = matchGlob(pattern, edgeCaseTest);

        // assert
        assert.ok(result);
      });

      test('it handles many consecutive globs', function (assert) {
        // arrange
        const pattern = '**is**a***test*';

        // act
        const result = matchGlob(pattern, edgeCaseTest);

        // assert
        assert.ok(result);
      });

      test('it handles white space between globs', function (assert) {
        // arrange
        const pattern = '* *';

        // act
        const result = matchGlob(pattern, edgeCaseTest);

        // assert
        assert.ok(result);
      });

      test('it handles a pattern of only globs', function (assert) {
        // arrange
        const pattern = '**********';

        // act
        const result = matchGlob(pattern, edgeCaseTest);

        // assert
        assert.ok(result);
      });

      test('it handles unicode characters', function (assert) {
        // arrange
        const pattern = `*Ѿ*`;

        // act
        const result = matchGlob(pattern, edgeCaseTest);

        // assert
        assert.ok(result);
      });

      test('it handles mixed ASCII codes', function (assert) {
        // arrange
        const pattern = `*is a ϗѾ *`;

        // act
        const result = matchGlob(pattern, edgeCaseTest);

        // assert
        assert.ok(result);
      });
    });

    module('non-matching edge cases', function () {
      const failingCases = [
        {
          case: 'test*',
          message: 'Implicit substring match',
        },
        {
          case: '*is',
          message: 'Parial match',
        },
        {
          case: '*no*',
          message: 'Globs without match between them',
        },
        {
          case: ' ',
          message: 'Plain white space',
        },
        {
          case: '* ',
          message: 'Trailing white space',
        },
        {
          case: ' *',
          message: 'Leading white space',
        },
        {
          case: '*ʤ*',
          message: 'Non-matching unicode',
        },
        {
          case: 'this*this is a test',
          message: 'Repeated prefix',
        },
      ];

      failingCases.forEach(({ case: failingPattern, message }) => {
        test('should fail the specified cases', function (assert) {
          const result = matchGlob(failingPattern, edgeCaseTest);
          assert.notOk(result, `${message} should not match.`);
        });
      });
    });
  });
});
