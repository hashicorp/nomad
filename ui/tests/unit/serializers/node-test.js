/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { run } from '@ember/runloop';
import NodeModel from 'nomad-ui/models/node';
import pushPayloadToStore from '../../utils/push-payload-to-store';
import { settled } from '@ember/test-helpers';

module('Unit | Serializer | Node', function (hooks) {
  setupTest(hooks);
  hooks.beforeEach(function () {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.serializerFor('node');
  });

  test('local store is culled to reflect the state of findAll requests', async function (assert) {
    const findAllResponse = [
      makeNode('1', 'One', '127.0.0.1:4646'),
      makeNode('2', 'Two', '127.0.0.2:4646'),
      makeNode('3', 'Three', '127.0.0.3:4646'),
    ];

    const payload = this.subject().normalizeFindAllResponse(
      this.store,
      NodeModel,
      findAllResponse
    );
    pushPayloadToStore(this.store, payload, NodeModel.modelName);

    assert.equal(
      payload.data.length,
      findAllResponse.length,
      'Each original record is returned in the response'
    );

    assert.equal(
      this.store.peekAll('node').filterBy('id').get('length'),
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
      newPayload = this.subject().normalizeFindAllResponse(
        this.store,
        NodeModel,
        newFindAllResponse
      );
    });
    pushPayloadToStore(this.store, newPayload, NodeModel.modelName);

    await settled();
    assert.equal(
      newPayload.data.length,
      newFindAllResponse.length,
      'Each new record is returned in the response'
    );

    assert.equal(
      this.store.peekAll('node').filterBy('id').get('length'),
      newFindAllResponse.length,
      'The node length in the store reflects the new response'
    );

    assert.notOk(
      this.store.peekAll('node').findBy('id', '1'),
      'Record One is no longer found'
    );
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
        HostVolumes: {
          one: {
            Name: 'one',
            ReadOnly: true,
          },
          two: {
            Name: 'two',
            ReadOnly: false,
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
            hostVolumes: [
              { name: 'one', readOnly: true },
              { name: 'two', readOnly: false },
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
            hostVolumes: [],
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
      name: 'Null hash values',
      in: {
        ID: 'test-node',
        Drivers: null,
        HostVolumes: null,
      },
      out: {
        data: {
          id: 'test-node',
          type: 'node',
          attributes: {
            hostVolumes: [],
            drivers: [],
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

  normalizationTestCases.forEach((testCase) => {
    test(`normalization: ${testCase.name}`, async function (assert) {
      assert.deepEqual(
        this.subject().normalize(NodeModel, testCase.in),
        testCase.out
      );
    });
  });
});
