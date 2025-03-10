/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { handleRouteRedirects } from 'nomad-ui/utils/route-redirector';
import sinon from 'sinon';

module('Unit | Utility | handle-route-redirects', function () {
  test('it handles different types of redirects correctly', function (assert) {
    assert.expect(7);

    const router = {
      replaceWith: sinon.spy(),
    };

    const testCases = [
      {
        name: 'exact match redirect',
        transition: {
          intent: { url: '/csi/volumes' },
          to: { queryParams: { region: 'global' } },
        },
        expectedPath: '/storage/volumes',
        expectedQueryParams: { region: 'global' },
      },
      {
        name: 'pattern match redirect',
        transition: {
          intent: { url: '/csi/volumes/my-volume' },
          to: { queryParams: { region: 'us-east' } },
        },
        expectedPath: '/storage/volumes/csi/my-volume',
        expectedQueryParams: { region: 'us-east' },
      },
      {
        name: 'startsWith redirect',
        transition: {
          intent: { url: '/csi' },
          to: { queryParams: {} },
        },
        expectedPath: '/storage',
        expectedQueryParams: {},
      },
      {
        name: 'no redirect needed',
        transition: {
          intent: { url: '/jobs' },
          to: { queryParams: {} },
        },
        expectedCalls: 0,
      },
    ];

    testCases
      .filter((testCase) => testCase.expectedCalls !== 0)
      .forEach((testCase) => {
        router.replaceWith.resetHistory();

        handleRouteRedirects(testCase.transition, router);
        assert.ok(
          router.replaceWith.calledOnce,
          `${testCase.name}: redirect occurred`
        );
        assert.ok(
          router.replaceWith.calledWith(testCase.expectedPath, {
            queryParams: testCase.expectedQueryParams,
          }),
          `${testCase.name}: redirected to correct path with query params`
        );
      });

    testCases
      .filter((testCase) => testCase.expectedCalls === 0)
      .forEach((testCase) => {
        router.replaceWith.resetHistory();

        handleRouteRedirects(testCase.transition, router);
        assert.notOk(
          router.replaceWith.called,
          `${testCase.name}: no redirect occurred`
        );
      });
  });

  test('it preserves query parameters during redirects', function (assert) {
    const router = {
      replaceWith: sinon.spy(),
    };

    const transition = {
      intent: { url: '/csi/volumes' },
      to: {
        queryParams: {
          region: 'global',
          namespace: 'default',
          foo: 'bar',
        },
      },
    };

    handleRouteRedirects(transition, router);

    assert.ok(
      router.replaceWith.calledWith('/storage/volumes', {
        queryParams: {
          region: 'global',
          namespace: 'default',
          foo: 'bar',
        },
      }),
      'All query parameters were preserved in the redirect'
    );
  });
});
