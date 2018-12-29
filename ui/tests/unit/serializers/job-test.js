import { test } from 'ember-qunit';
import JobModel from 'nomad-ui/models/job';
import moduleForSerializer from '../../helpers/module-for-serializer';

moduleForSerializer('job', 'Unit | Serializer | Job', {
  needs: [
    'serializer:job',
    'model:task-group-summary',
    'model:task-group',
    'transform:fragment-array',
  ],
});

test('`default` is used as the namespace in the job ID when there is no namespace in the payload', function(assert) {
  const original = {
    ID: 'example',
    Name: 'example',
  };

  const { data } = this.subject().normalize(JobModel, original);
  assert.equal(data.id, JSON.stringify([data.attributes.name, 'default']));
});

test('The ID of the record is a composite of both the name and the namespace', function(assert) {
  const original = {
    ID: 'example',
    Name: 'example',
    Namespace: 'special-namespace',
  };

  const { data } = this.subject().normalize(JobModel, original);
  assert.equal(
    data.id,
    JSON.stringify([data.attributes.name, data.relationships.namespace.data.id])
  );
});
