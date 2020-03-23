import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import VolumeModel from 'nomad-ui/models/volume';

module('Unit | Serializer | Job', function(hooks) {
  setupTest(hooks);
  hooks.beforeEach(function() {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.serializerFor('volume');
  });

  const normalizationTestCases = [
    {
      name:
        '`default` is used as the namespace in the volume ID when there is no namespace in the payload',
      in: {},
      out: {},
    },
    {
      name: 'The ID of the record is a composite of both the name and the namespace',
      in: {},
      out: {},
    },
    {
      name:
        'Allocations are interpreted as embedded records and are properly normalized into included resources in a JSON API shape',
      in: {},
      out: {},
    },
  ];

  normalizationTestCases.forEach(testCase => {
    test(`normalization: ${testCase.name}`, async function(assert) {
      assert.deepEqual(this.subject().normalize(VolumeModel, testCase.in), testCase.out);
    });
  });
});
