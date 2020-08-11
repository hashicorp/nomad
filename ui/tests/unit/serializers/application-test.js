import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import ApplicationSerializer from 'nomad-ui/serializers/application';

import Model from 'ember-data/model';
import attr from 'ember-data/attr';

class TestSerializer extends ApplicationSerializer {
  arrayNullOverrides = ['Things'];
  mapToArray = ['ArrayableMap'];
}

class TestModel extends Model {
  @attr() things;
  @attr() arrayableMap;
}

module('Unit | Serializer | Application', function(hooks) {
  setupTest(hooks);

  hooks.beforeEach(function() {
    this.store = this.owner.lookup('service:store');
    this.owner.register('model:test', TestModel);
    this.owner.register('serializer:test', TestSerializer);

    this.subject = () => this.store.serializerFor('test');
  });

  const normalizationTestCases = [
    {
      name: 'Null array and map',
      in: {
        ID: 'test-test',
        Things: null,
        ArrayableMap: null,
      },
      out: {
        data: {
          id: 'test-test',
          attributes: {
            things: [],
            arrayableMap: [],
          },
          relationships: {},
          type: 'test',
        },
      },
    },
    {
      name: 'Non-null array and map',
      in: {
        ID: 'test-test',
        Things: [1, 2, 3],
        ArrayableMap: {
          a: { Order: 1 },
          b: { Order: 2 },
          'c.d': { Order: 3 },
        },
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
          },
          relationships: {},
          type: 'test',
        },
      },
    },
  ];

  normalizationTestCases.forEach(testCase => {
    test(`normalization: ${testCase.name}`, async function(assert) {
      assert.deepEqual(this.subject().normalize(TestModel, testCase.in), testCase.out);
    });
  });
});
