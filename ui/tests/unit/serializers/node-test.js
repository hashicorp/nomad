import { run } from '@ember/runloop';
import { test } from 'ember-qunit';
import wait from 'ember-test-helpers/wait';
import NodeModel from 'nomad-ui/models/node';
import moduleForSerializer from '../../helpers/module-for-serializer';
import pushPayloadToStore from '../../utils/push-payload-to-store';

moduleForSerializer('node', 'Unit | Serializer | Node', {
  needs: ['serializer:node', 'service:config', 'transform:fragment', 'model:allocation'],
});

test('local store is culled to reflect the state of findAll requests', function(assert) {
  const findAllResponse = [
    makeNode('1', 'One', '127.0.0.1:4646'),
    makeNode('2', 'Two', '127.0.0.2:4646'),
    makeNode('3', 'Three', '127.0.0.3:4646'),
  ];

  const payload = this.subject().normalizeFindAllResponse(this.store, NodeModel, findAllResponse);
  pushPayloadToStore(this.store, payload, NodeModel.modelName);

  assert.equal(
    payload.data.length,
    findAllResponse.length,
    'Each original record is returned in the response'
  );

  assert.equal(
    this.store
      .peekAll('node')
      .filterBy('id')
      .get('length'),
    findAllResponse.length,
    'Each original record is now in the store'
  );

  const newFindAllResponse = [
    makeNode('2', 'Two', '127.0.0.2:4646'),
    makeNode('3', 'Three', '127.0.0.3:4646'),
    makeNode('4', 'Four', '127.0.0.4:4646'),
  ];

  let newPayload;
  run(() => {
    newPayload = this.subject().normalizeFindAllResponse(this.store, NodeModel, newFindAllResponse);
  });
  pushPayloadToStore(this.store, newPayload, NodeModel.modelName);

  return wait().then(() => {
    assert.equal(
      newPayload.data.length,
      newFindAllResponse.length,
      'Each new record is returned in the response'
    );

    assert.equal(
      this.store
        .peekAll('node')
        .filterBy('id')
        .get('length'),
      newFindAllResponse.length,
      'The node length in the store reflects the new response'
    );

    assert.notOk(this.store.peekAll('node').findBy('id', '1'), 'Record One is no longer found');
  });
});

function makeNode(id, name, ip) {
  return { ID: id, Name: name, HTTPAddr: ip };
}
