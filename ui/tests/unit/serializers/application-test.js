/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import ApplicationSerializer from 'nomad-ui/serializers/application';

import Model from '@ember-data/model';
import { attr } from '@ember-data/model';
import classic from 'ember-classic-decorator';

@classic
class TestSerializer extends ApplicationSerializer {
  arrayNullOverrides = ['Things'];

  mapToArray = [
    'ArrayableMap',
    {
      beforeName: 'OriginalNameArrayableMap',
      afterName: 'RenamedArrayableMap',
    },
  ];

  separateNanos = ['Time'];
}

class TestModel extends Model {
  @attr() things;

  @attr() arrayableMap;
  @attr() renamedArrayableMap;

  @attr() time;
  @attr() timeNanos;
}

module('Unit | Serializer | Application', function (hooks) {
  setupTest(hooks);

  hooks.beforeEach(function () {
    this.store = this.owner.lookup('service:store');
    this.owner.register('model:test', TestModel);
    this.owner.register('serializer:test', TestSerializer);

    this.subject = () => this.store.serializerFor('test');
  });

  const normalizationTestCases = [
    {
      name: 'Null array and maps',
      in: {
        ID: 'test-test',
        Things: null,
        ArrayableMap: null,
        OriginalNameArrayableMap: null,
        Time: 1607839992000100000,
      },
      out: {
        data: {
          id: 'test-test',
          attributes: {
            things: [],
            arrayableMap: [],
            renamedArrayableMap: [],
            time: 1607839992000,
            timeNanos: 100096,
          },
          relationships: {},
          type: 'test',
        },
      },
    },
    {
      name: 'Non-null array and maps',
      in: {
        ID: 'test-test',
        Things: [1, 2, 3],
        ArrayableMap: {
          b: { Order: 2 },
          a: { Order: 1 },
          'c.d': { Order: 3 },
        },
        OriginalNameArrayableMap: {
          a: { X: 1 },
        },
        Time: 1607839992000100000,
        SomethingExtra: 'xyz',
      },
      out: {
        data: {
          id: 'test-test',
          attributes: {
            things: [1, 2, 3],
            arrayableMap: [
              { Name: 'a', Order: 1 },
              { Name: 'b', Order: 2 },
              { Name: 'c.d', Order: 3 },
            ],
            renamedArrayableMap: [{ Name: 'a', X: 1 }],
            time: 1607839992000,
            timeNanos: 100096,
          },
          relationships: {},
          type: 'test',
        },
      },
    },
  ];

  normalizationTestCases.forEach((testCase) => {
    test(`normalization: ${testCase.name}`, async function (assert) {
      assert.deepEqual(
        this.subject().normalize(TestModel, testCase.in),
        testCase.out
      );
    });
  });
});
