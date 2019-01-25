import { run } from '@ember/runloop';
import { test } from 'ember-qunit';
import wait from 'ember-test-helpers/wait';
import NodeModel from 'nomad-ui/models/node';
import moduleForSerializer from '../../helpers/module-for-serializer';
import pushPayloadToStore from '../../utils/push-payload-to-store';

moduleForSerializer('node', 'Unit | Serializer | Node', {
  needs: [
    'adapter:application',
    'service:config',
    'serializer:node',
    'service:system',
    'service:token',
    'transform:fragment',
    'transform:fragment-array',
    'model:node-attributes',
    'model:resources',
    'model:drain-strategy',
    'model:node-driver',
    'model:node-event',
    'model:allocation',
    'model:job',
  ],
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

const normalizationTestCases = [
  {
    name: 'Normal',
    in: {
      ID: 'test-node',
      HTTPAddr: '867.53.0.9:4646',
      Drain: false,
      Drivers: {
        docker: {
          Detected: true,
          Healthy: false,
        },
      },
    },
    out: {
      data: {
        id: 'test-node',
        type: 'node',
        attributes: {
          isDraining: false,
          httpAddr: '867.53.0.9:4646',
          drivers: [
            {
              name: 'docker',
              detected: true,
              healthy: false,
            },
          ],
        },
        relationships: {
          allocations: {
            links: {
              related: '/v1/node/test-node/allocations',
            },
          },
        },
      },
    },
  },

  {
    name: 'Dots in driver names',
    in: {
      ID: 'test-node',
      HTTPAddr: '867.53.0.9:4646',
      Drain: false,
      Drivers: {
        'my.driver': {
          Detected: true,
          Healthy: false,
        },
        'my.other.driver': {
          Detected: false,
          Healthy: false,
        },
      },
    },
    out: {
      data: {
        id: 'test-node',
        type: 'node',
        attributes: {
          isDraining: false,
          httpAddr: '867.53.0.9:4646',
          drivers: [
            {
              name: 'my.driver',
              detected: true,
              healthy: false,
            },
            {
              name: 'my.other.driver',
              detected: false,
              healthy: false,
            },
          ],
        },
        relationships: {
          allocations: {
            links: {
              related: '/v1/node/test-node/allocations',
            },
          },
        },
      },
    },
  },
];

normalizationTestCases.forEach(testCase => {
  test(`normalization: ${testCase.name}`, function(assert) {
    assert.deepEqual(this.subject().normalize(NodeModel, testCase.in), testCase.out);
  });
});
