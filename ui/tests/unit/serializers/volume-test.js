/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import VolumeModel from 'nomad-ui/models/volume';

module('Unit | Serializer | Volume', function (hooks) {
  setupTest(hooks);
  hooks.beforeEach(function () {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.serializerFor('volume');
  });

  // Set the milliseconds to avoid possible floating point precision
  // issue that arises from converting to nanos and back.
  const REF_DATE = new Date();
  REF_DATE.setMilliseconds(0);

  const normalizationTestCases = [
    {
      name: '`default` is used as the namespace in the volume ID when there is no namespace in the payload',
      in: {
        ID: 'volume-id',
        Name: 'volume-id',
        PluginID: 'plugin-1',
        ExternalID: 'external-uuid',
        Topologies: {},
        AccessMode: 'access-this-way',
        AttachmentMode: 'attach-this-way',
        Schedulable: true,
        Provider: 'abc.123',
        Version: '1.0.29',
        ControllerRequired: true,
        ControllersHealthy: 1,
        ControllersExpected: 1,
        NodesHealthy: 1,
        NodesExpected: 2,
        CreateIndex: 1,
        ModifyIndex: 38,
        WriteAllocs: {},
        ReadAllocs: {},
      },
      out: {
        data: {
          id: '["csi/volume-id","default"]',
          type: 'volume',
          attributes: {
            plainId: 'volume-id',
            name: 'volume-id',
            externalId: 'external-uuid',
            topologies: {},
            accessMode: 'access-this-way',
            attachmentMode: 'attach-this-way',
            schedulable: true,
            provider: 'abc.123',
            version: '1.0.29',
            controllerRequired: true,
            controllersHealthy: 1,
            controllersExpected: 1,
            nodesHealthy: 1,
            nodesExpected: 2,
            createIndex: 1,
            modifyIndex: 38,
          },
          relationships: {
            plugin: {
              data: {
                id: 'csi/plugin-1',
                type: 'plugin',
              },
            },
            readAllocations: {
              data: [],
            },
            writeAllocations: {
              data: [],
            },
          },
        },
        included: [],
      },
    },

    {
      name: 'The ID of the record is a composite of both the name and the namespace',
      in: {
        ID: 'volume-id',
        Name: 'volume-id',
        Namespace: 'namespace-2',
        PluginID: 'plugin-1',
        ExternalID: 'external-uuid',
        Topologies: {},
        AccessMode: 'access-this-way',
        AttachmentMode: 'attach-this-way',
        Schedulable: true,
        Provider: 'abc.123',
        Version: '1.0.29',
        ControllerRequired: true,
        ControllersHealthy: 1,
        ControllersExpected: 1,
        NodesHealthy: 1,
        NodesExpected: 2,
        CreateIndex: 1,
        ModifyIndex: 38,
        WriteAllocs: {},
        ReadAllocs: {},
      },
      out: {
        data: {
          id: '["csi/volume-id","namespace-2"]',
          type: 'volume',
          attributes: {
            plainId: 'volume-id',
            name: 'volume-id',
            externalId: 'external-uuid',
            topologies: {},
            accessMode: 'access-this-way',
            attachmentMode: 'attach-this-way',
            schedulable: true,
            provider: 'abc.123',
            version: '1.0.29',
            controllerRequired: true,
            controllersHealthy: 1,
            controllersExpected: 1,
            nodesHealthy: 1,
            nodesExpected: 2,
            createIndex: 1,
            modifyIndex: 38,
          },
          relationships: {
            plugin: {
              data: {
                id: 'csi/plugin-1',
                type: 'plugin',
              },
            },
            namespace: {
              data: {
                id: 'namespace-2',
                type: 'namespace',
              },
            },
            readAllocations: {
              data: [],
            },
            writeAllocations: {
              data: [],
            },
          },
        },
        included: [],
      },
    },

    {
      name: 'Allocations are interpreted as embedded records and are properly normalized into included resources in a JSON API shape',
      in: {
        ID: 'volume-id',
        Name: 'volume-id',
        Namespace: 'namespace-2',
        PluginID: 'plugin-1',
        ExternalID: 'external-uuid',
        Topologies: {},
        AccessMode: 'access-this-way',
        AttachmentMode: 'attach-this-way',
        Schedulable: true,
        Provider: 'abc.123',
        Version: '1.0.29',
        ControllerRequired: true,
        ControllersHealthy: 1,
        ControllersExpected: 1,
        NodesHealthy: 1,
        NodesExpected: 2,
        CreateIndex: 1,
        ModifyIndex: 38,
        Allocations: [
          {
            ID: 'alloc-id-1',
            TaskGroup: 'foobar',
            CreateTime: +REF_DATE * 1000000,
            ModifyTime: +REF_DATE * 1000000,
            JobID: 'the-job',
            Namespace: 'namespace-2',
          },
          {
            ID: 'alloc-id-2',
            TaskGroup: 'write-here',
            CreateTime: +REF_DATE * 1000000,
            ModifyTime: +REF_DATE * 1000000,
            JobID: 'the-job',
            Namespace: 'namespace-2',
          },
          {
            ID: 'alloc-id-3',
            TaskGroup: 'look-if-you-must',
            CreateTime: +REF_DATE * 1000000,
            ModifyTime: +REF_DATE * 1000000,
            JobID: 'the-job',
            Namespace: 'namespace-2',
          },
        ],
        WriteAllocs: {
          'alloc-id-1': null,
          'alloc-id-2': null,
        },
        ReadAllocs: {
          'alloc-id-3': null,
        },
      },
      out: {
        data: {
          id: '["csi/volume-id","namespace-2"]',
          type: 'volume',
          attributes: {
            plainId: 'volume-id',
            name: 'volume-id',
            externalId: 'external-uuid',
            topologies: {},
            accessMode: 'access-this-way',
            attachmentMode: 'attach-this-way',
            schedulable: true,
            provider: 'abc.123',
            version: '1.0.29',
            controllerRequired: true,
            controllersHealthy: 1,
            controllersExpected: 1,
            nodesHealthy: 1,
            nodesExpected: 2,
            createIndex: 1,
            modifyIndex: 38,
          },
          relationships: {
            plugin: {
              data: {
                id: 'csi/plugin-1',
                type: 'plugin',
              },
            },
            namespace: {
              data: {
                id: 'namespace-2',
                type: 'namespace',
              },
            },
            readAllocations: {
              data: [{ type: 'allocation', id: 'alloc-id-3' }],
            },
            writeAllocations: {
              data: [
                { type: 'allocation', id: 'alloc-id-1' },
                { type: 'allocation', id: 'alloc-id-2' },
              ],
            },
          },
        },
        included: [
          {
            id: 'alloc-id-1',
            type: 'allocation',
            attributes: {
              createTime: REF_DATE,
              modifyTime: REF_DATE,
              namespace: 'namespace-2',
              taskGroupName: 'foobar',
              wasPreempted: false,
              states: [],
              allocationTaskGroup: null,
            },
            relationships: {
              followUpEvaluation: {
                data: null,
              },
              job: {
                data: { type: 'job', id: '["the-job","namespace-2"]' },
              },
              nextAllocation: {
                data: null,
              },
              previousAllocation: {
                data: null,
              },
              preemptedAllocations: {
                data: [],
              },
              preemptedByAllocation: {
                data: null,
              },
            },
          },
          {
            id: 'alloc-id-2',
            type: 'allocation',
            attributes: {
              createTime: REF_DATE,
              modifyTime: REF_DATE,
              namespace: 'namespace-2',
              taskGroupName: 'write-here',
              wasPreempted: false,
              states: [],
              allocationTaskGroup: null,
            },
            relationships: {
              followUpEvaluation: {
                data: null,
              },
              job: {
                data: { type: 'job', id: '["the-job","namespace-2"]' },
              },
              nextAllocation: {
                data: null,
              },
              previousAllocation: {
                data: null,
              },
              preemptedAllocations: {
                data: [],
              },
              preemptedByAllocation: {
                data: null,
              },
            },
          },
          {
            id: 'alloc-id-3',
            type: 'allocation',
            attributes: {
              createTime: REF_DATE,
              modifyTime: REF_DATE,
              namespace: 'namespace-2',
              taskGroupName: 'look-if-you-must',
              wasPreempted: false,
              states: [],
              allocationTaskGroup: null,
            },
            relationships: {
              followUpEvaluation: {
                data: null,
              },
              job: {
                data: { type: 'job', id: '["the-job","namespace-2"]' },
              },
              nextAllocation: {
                data: null,
              },
              previousAllocation: {
                data: null,
              },
              preemptedAllocations: {
                data: [],
              },
              preemptedByAllocation: {
                data: null,
              },
            },
          },
        ],
      },
    },
  ];

  normalizationTestCases.forEach((testCase) => {
    test(`normalization: ${testCase.name}`, async function (assert) {
      assert.deepEqual(
        this.subject().normalize(VolumeModel, testCase.in),
        testCase.out
      );
    });
  });
});
