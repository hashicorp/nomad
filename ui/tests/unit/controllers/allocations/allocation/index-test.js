/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Controller | allocations/allocation/index', function (hooks) {
  setupTest(hooks);

  module('#serviceHealthStatuses', function () {
    test('it groups health service data by service name', function (assert) {
      let controller = this.owner.lookup(
        'controller:allocations/allocation/index'
      );
      controller.set('model', JSON.parse(JSON.stringify(Allocation)));

      const groupFakePy = {
        refID: 'fakepy-group-fake-py',
        statuses: {
          success: 1,
          failure: 1,
          pending: 0,
        },
      };
      const taskFakePy = {
        refID: 'http.server-task-fake-py',
        statuses: {
          success: 2,
          failure: 2,
          pending: 0,
        },
      };
      const pender = {
        refID: 'http.server-pender',
        statuses: {
          success: 0,
          failure: 0,
          pending: 1,
        },
      };

      assert.equal(
        controller.servicesWithHealthChecks
          .findBy('refID', groupFakePy.refID)
          .healthChecks.filter((check) => check.Status === 'success').length,
        groupFakePy.statuses['success']
      );
      assert.equal(
        controller.servicesWithHealthChecks
          .findBy('refID', groupFakePy.refID)
          .healthChecks.filter((check) => check.Status === 'failure').length,
        groupFakePy.statuses['failure']
      );
      assert.equal(
        controller.servicesWithHealthChecks
          .findBy('refID', groupFakePy.refID)
          .healthChecks.filter((check) => check.Status === 'pending').length,
        groupFakePy.statuses['pending']
      );

      assert.equal(
        controller.servicesWithHealthChecks
          .findBy('refID', taskFakePy.refID)
          .healthChecks.filter((check) => check.Status === 'success').length,
        taskFakePy.statuses['success']
      );
      assert.equal(
        controller.servicesWithHealthChecks
          .findBy('refID', taskFakePy.refID)
          .healthChecks.filter((check) => check.Status === 'failure').length,
        taskFakePy.statuses['failure']
      );
      assert.equal(
        controller.servicesWithHealthChecks
          .findBy('refID', taskFakePy.refID)
          .healthChecks.filter((check) => check.Status === 'pending').length,
        taskFakePy.statuses['pending']
      );

      assert.equal(
        controller.servicesWithHealthChecks
          .findBy('refID', pender.refID)
          .healthChecks.filter((check) => check.Status === 'success').length,
        pender.statuses['success']
      );
      assert.equal(
        controller.servicesWithHealthChecks
          .findBy('refID', pender.refID)
          .healthChecks.filter((check) => check.Status === 'failure').length,
        pender.statuses['failure']
      );
      assert.equal(
        controller.servicesWithHealthChecks
          .findBy('refID', pender.refID)
          .healthChecks.filter((check) => check.Status === 'pending').length,
        pender.statuses['pending']
      );
    });

    test('it handles duplicate names', async function (assert) {
      let controller = this.owner.lookup(
        'controller:allocations/allocation/index'
      );
      controller.set('model', JSON.parse(JSON.stringify(Allocation)));

      const groupDupe = {
        refID: 'fakepy-duper',
        statuses: {
          success: 1,
          failure: 0,
          pending: 0,
        },
      };
      const taskDupe = {
        refID: 'http.server-duper',
        statuses: {
          success: 0,
          failure: 1,
          pending: 0,
        },
      };

      assert.equal(
        controller.servicesWithHealthChecks
          .findBy('refID', groupDupe.refID)
          .healthChecks.filter((check) => check.Status === 'success').length,
        groupDupe.statuses['success']
      );
      assert.equal(
        controller.servicesWithHealthChecks
          .findBy('refID', groupDupe.refID)
          .healthChecks.filter((check) => check.Status === 'failure').length,
        groupDupe.statuses['failure']
      );
      assert.equal(
        controller.servicesWithHealthChecks
          .findBy('refID', groupDupe.refID)
          .healthChecks.filter((check) => check.Status === 'pending').length,
        groupDupe.statuses['pending']
      );

      assert.equal(
        controller.servicesWithHealthChecks
          .findBy('refID', taskDupe.refID)
          .healthChecks.filter((check) => check.Status === 'success').length,
        taskDupe.statuses['success']
      );
      assert.equal(
        controller.servicesWithHealthChecks
          .findBy('refID', taskDupe.refID)
          .healthChecks.filter((check) => check.Status === 'failure').length,
        taskDupe.statuses['failure']
      );
      assert.equal(
        controller.servicesWithHealthChecks
          .findBy('refID', taskDupe.refID)
          .healthChecks.filter((check) => check.Status === 'pending').length,
        taskDupe.statuses['pending']
      );
    });
  });
});

// Using var to hoist this variable to the top of the module
var Allocation = {
  namespace: 'default',
  name: 'my-alloc',
  taskGroup: {
    name: 'fakepy',
    count: 3,
    services: [
      {
        Name: 'group-fake-py',
        refID: 'fakepy-group-fake-py',
        PortLabel: 'http',
        Tags: [],
        OnUpdate: 'require_healthy',
        Provider: 'nomad',
        Connect: null,
        GroupName: 'fakepy',
        TaskName: '',
        healthChecks: [],
      },
      {
        Name: 'duper',
        refID: 'fakepy-duper',
        PortLabel: 'http',
        Tags: [],
        OnUpdate: 'require_healthy',
        Provider: 'nomad',
        Connect: null,
        GroupName: 'fakepy',
        TaskName: '',
        healthChecks: [],
      },
    ],
  },
  allocatedResources: {
    Cpu: 100,
    Memory: 300,
    MemoryMax: null,
    Disk: 0,
    Iops: null,
    Networks: [
      {
        Device: '',
        CIDR: '',
        IP: '127.0.0.1',
        Mode: 'host',
        MBits: 0,
        Ports: [
          {
            name: 'http',
            port: 22308,
            to: 0,
            isDynamic: true,
          },
        ],
      },
    ],
    Ports: [
      {
        HostIP: '127.0.0.1',
        Label: 'http',
        To: 0,
        Value: 22308,
      },
    ],
  },
  healthChecks: {
    c97fda942e772b43a5a537e5b0c8544c: {
      Check: 'service: "task-fake-py" check',
      Alloc: 'my-alloc',
      Group: 'trying-multi-dupes.fakepy[1]',
      ID: 'c97fda942e772b43a5a537e5b0c8544c',
      Mode: 'healthiness',
      Output: 'nomad: http ok',
      Service: 'task-fake-py',
      Status: 'success',
      StatusCode: 200,
      Task: 'http.server',
      Timestamp: 1662131947,
    },
    '2e1bfc8ecc485ee86b972ae08e890152': {
      Check: 'task-happy',
      Alloc: 'my-alloc',
      Group: 'trying-multi-dupes.fakepy[1]',
      ID: '2e1bfc8ecc485ee86b972ae08e890152',
      Mode: 'healthiness',
      Output: 'nomad: http ok',
      Service: 'task-fake-py',
      Status: 'success',
      StatusCode: 200,
      Task: 'http.server',
      Timestamp: 1662131949,
    },
    '6162723ab20b268c25eda69b400dc9c6': {
      Check: 'task-sad',
      Alloc: 'my-alloc',
      Group: 'trying-multi-dupes.fakepy[1]',
      ID: '6162723ab20b268c25eda69b400dc9c6',
      Mode: 'healthiness',
      Output:
        '<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN"\n        "http://www.w3.org/TR/html4/strict.dtd">\n<html>\n    <head>\n        <meta http-equiv="Content-Type" content="text/html;charset=utf-8">\n        <title>Error response</title>\n    </head>\n    <body>\n        <h1>Error response</h1>\n        <p>Error code: 404</p>\n        <p>Message: File not found.</p>\n        <p>Error code explanation: HTTPStatus.NOT_FOUND - Nothing matches the given URI.</p>\n    </body>\n</html>\n',
      Service: 'task-fake-py',
      Status: 'failure',
      StatusCode: 404,
      Task: 'http.server',
      Timestamp: 1662131936,
    },
    a4a7050175a2b236edcf613cb3563753: {
      Check: 'task-sad2',
      Alloc: 'my-alloc',
      Group: 'trying-multi-dupes.fakepy[1]',
      ID: 'a4a7050175a2b236edcf613cb3563753',
      Mode: 'healthiness',
      Output:
        '<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN"\n        "http://www.w3.org/TR/html4/strict.dtd">\n<html>\n    <head>\n        <meta http-equiv="Content-Type" content="text/html;charset=utf-8">\n        <title>Error response</title>\n    </head>\n    <body>\n        <h1>Error response</h1>\n        <p>Error code: 404</p>\n        <p>Message: File not found.</p>\n        <p>Error code explanation: HTTPStatus.NOT_FOUND - Nothing matches the given URI.</p>\n    </body>\n</html>\n',
      Service: 'task-fake-py',
      Status: 'failure',
      StatusCode: 404,
      Task: 'http.server',
      Timestamp: 1662131936,
    },
    '2dfe58eb841bdfa704f0ae9ef5b5af5e': {
      Check: 'tcp_probe',
      Alloc: 'my-alloc',
      Group: 'trying-multi-dupes.fakepy[1]',
      ID: '2dfe58eb841bdfa704f0ae9ef5b5af5e',
      Mode: 'readiness',
      Output: 'nomad: tcp ok',
      Service: 'web',
      Status: 'success',
      Task: 'http.server',
      Timestamp: 1662131949,
    },
    '69021054964f4c461b3c4c4f456e16a8': {
      Check: 'happy',
      Alloc: 'my-alloc',
      Group: 'trying-multi-dupes.fakepy[1]',
      ID: '69021054964f4c461b3c4c4f456e16a8',
      Mode: 'healthiness',
      Output: 'nomad: http ok',
      Service: 'group-fake-py',
      Status: 'success',
      StatusCode: 200,
      Timestamp: 1662131949,
    },
    '913f5b725ceecdd5ff48a9a51ddf8513': {
      Check: 'sad',
      Alloc: 'my-alloc',
      Group: 'trying-multi-dupes.fakepy[1]',
      ID: '913f5b725ceecdd5ff48a9a51ddf8513',
      Mode: 'healthiness',
      Output:
        '<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN"\n        "http://www.w3.org/TR/html4/strict.dtd">\n<html>\n    <head>\n        <meta http-equiv="Content-Type" content="text/html;charset=utf-8">\n        <title>Error response</title>\n    </head>\n    <body>\n        <h1>Error response</h1>\n        <p>Error code: 404</p>\n        <p>Message: File not found.</p>\n        <p>Error code explanation: HTTPStatus.NOT_FOUND - Nothing matches the given URI.</p>\n    </body>\n</html>\n',
      Service: 'group-fake-py',
      Status: 'failure',
      StatusCode: 404,
      Timestamp: 1662131936,
    },
    bloop: {
      Check: 'is-alive',
      Alloc: 'my-alloc',
      Group: 'trying-multi-dupes.fakepy[1]',
      ID: 'bloop',
      Mode: 'healthiness',
      Service: 'pender',
      Status: 'pending',
      Task: 'http.server',
      Timestamp: 1662131947,
    },
    'group-dupe': {
      Check: 'is-alive',
      Alloc: 'my-alloc',
      Group: 'trying-multi-dupes.fakepy[1]',
      ID: 'group-dupe',
      Mode: 'healthiness',
      Service: 'duper',
      Status: 'success',
      Task: '',
      Timestamp: 1662131947,
    },
    'task-dupe': {
      Check: 'is-alive',
      Alloc: 'my-alloc',
      Group: 'trying-multi-dupes.fakepy[1]',
      ID: 'task-dupe',
      Mode: 'healthiness',
      Service: 'duper',
      Status: 'failure',
      Task: 'http.server',
      Timestamp: 1662131947,
    },
  },
  id: 'my-alloc',
  states: [
    {
      Name: 'http.server',
      task: {
        name: 'http.server',
        driver: 'raw_exec',
        kind: '',
        meta: null,
        lifecycle: null,
        reservedMemory: 300,
        reservedMemoryMax: 0,
        reservedCPU: 100,
        reservedDisk: 0,
        reservedEphemeralDisk: 300,
        services: [
          {
            Name: 'task-fake-py',
            PortLabel: 'http',
            refID: 'http.server-task-fake-py',
            Tags: [
              'long',
              'and',
              'arbitrary',
              'list',
              'of',
              'tags',
              'arbitrary',
            ],
            OnUpdate: 'require_healthy',
            Provider: 'nomad',
            Connect: null,
            TaskName: 'http.server',
            healthChecks: [],
          },
          {
            Name: 'pender',
            refID: 'http.server-pender',
            PortLabel: 'http',
            Tags: ['lol', 'lmao'],
            OnUpdate: 'require_healthy',
            Provider: 'nomad',
            Connect: null,
            TaskName: 'http.server',
            healthChecks: [],
          },
          {
            Name: 'web',
            refID: 'http.server-web',
            PortLabel: 'http',
            Tags: ['lol', 'lmao'],
            OnUpdate: 'require_healthy',
            Provider: 'nomad',
            Connect: null,
            TaskName: 'http.server',
            healthChecks: [],
          },
          {
            Name: 'duper',
            refID: 'http.server-duper',
            PortLabel: 'http',
            Tags: ['lol', 'lmao'],
            OnUpdate: 'require_healthy',
            Provider: 'nomad',
            Connect: null,
            TaskName: 'http.server',
            healthChecks: [],
          },
        ],
        volumeMounts: null,
      },
    },
  ],
  rescheduleEvents: [],
  job: '["trying-multi-dupes","default"]',
  node: '5d33384d-8d0f-6a65-743c-2fcc1871b13e',
  previousAllocation: null,
  nextAllocation: null,
  preemptedByAllocation: null,
  followUpEvaluation: null,
};
