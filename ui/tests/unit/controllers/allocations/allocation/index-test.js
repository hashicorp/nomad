import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Controller | allocations/allocation/index', function (hooks) {
  setupTest(hooks);

  module('#serviceHealthStatuses', function () {
    test('it groups health service data by service name', function (assert) {
      let controller = this.owner.lookup(
        'controller:allocations/allocation/index'
      );

      controller.set('model', Allocation);

      const result = new Map();
      result.set('fake-py', {
        failure: 1,
        success: 1,
      });
      result.set('task-fake-py', {
        failure: 1,
        success: 1,
      });
      result.set('web', {
        success: 1,
      });

      const fakePy = controller.serviceHealthStatuses.get('fake-py');
      const taskFakePy = controller.serviceHealthStatuses.get('task-fake-py');
      const web = controller.serviceHealthStatuses.get('web');

      assert.deepEqual(
        fakePy,
        result.get('fake-py'),
        'Service Health Check data is transformed and grouped by Service name'
      );
      assert.deepEqual(
        taskFakePy,
        result.get('task-fake-py'),
        'Service Health Check data is transformed and grouped by Service name'
      );
      assert.deepEqual(
        web,
        result.get('web'),
        'Service Health Check data is transformed and grouped by Service name'
      );
    });

    test('it handles duplicate names', function (assert) {
      let controller = this.owner.lookup(
        'controller:allocations/allocation/index'
      );

      const dupeTaskLevelService =
        Allocation.allocationTaskGroup.Tasks[0].Services[0];
      dupeTaskLevelService.Name = 'fake-py@task';
      dupeTaskLevelService.isTaskLevel = true;

      const healthChecks = Allocation.healthChecks;
      healthChecks['73ad9b936fb3f3cc4d7f62a1aab6de53'].Service = 'fake-py';
      healthChecks['19421ef816ae0d3eeeb81697bce0e261'].Service = 'fake-py';

      controller.set('model', Allocation);

      const result = new Map();
      result.set('fake-py', {
        failure: 1,
        success: 1,
      });
      result.set('fake-py@task', {
        failure: 1,
        success: 1,
      });
      result.set('web@task', {
        success: 1,
      });

      const fakePy = controller.serviceHealthStatuses.get('fake-py');
      const taskFakePy = controller.serviceHealthStatuses.get('fake-py@task');
      const web = controller.serviceHealthStatuses.get('web@task');

      assert.deepEqual(
        fakePy,
        result.get('fake-py'),
        'Service Health Check data is transformed and grouped by Service name'
      );
      assert.deepEqual(
        taskFakePy,
        result.get('fake-py@task'),
        'Service Health Check data is transformed and grouped by Service name'
      );
      assert.deepEqual(
        web,
        result.get('web@task'),
        'Service Health Check data is transformed and grouped by Service name'
      );
    });
  });
});

// Using var to hoist this variable to the top of the module
var Allocation = {
  namespace: 'default',
  name: 'trying-multi-dupes.fakepy[1]',
  taskGroupName: 'fakepy',
  resources: {
    Cpu: null,
    Memory: null,
    MemoryMax: null,
    Disk: null,
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
    Ports: [],
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
  jobVersion: 0,
  modifyIndex: 31,
  modifyTime: '2022-08-29T14:13:57.761Z',
  createIndex: 15,
  createTime: '2022-08-29T14:08:57.587Z',
  clientStatus: 'running',
  desiredStatus: 'run',
  healthChecks: {
    '93a090236c79d964d1381cb218efc0f5': {
      Check: 'happy',
      Group: 'trying-multi-dupes.fakepy[1]',
      ID: '93a090236c79d964d1381cb218efc0f5',
      Mode: 'healthiness',
      Output: 'nomad: http ok',
      Service: 'fake-py',
      Status: 'success',
      StatusCode: 200,
      Timestamp: 1661787992,
    },
    '4b5daa12d4159bcb367aac65548f48f4': {
      Check: 'sad',
      Group: 'trying-multi-dupes.fakepy[1]',
      ID: '4b5daa12d4159bcb367aac65548f48f4',
      Mode: 'healthiness',
      Output:
        '<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN"\n        "http://www.w3.org/TR/html4/strict.dtd">\n<html>\n    <head>\n        <meta http-equiv="Content-Type" content="text/html;charset=utf-8">\n        <title>Error response</title>\n    </head>\n    <body>\n        <h1>Error response</h1>\n        <p>Error code: 404</p>\n        <p>Message: File not found.</p>\n        <p>Error code explanation: HTTPStatus.NOT_FOUND - Nothing matches the given URI.</p>\n    </body>\n</html>\n',
      Service: 'fake-py',
      Status: 'failure',
      StatusCode: 404,
      Timestamp: 1661787965,
    },
    '73ad9b936fb3f3cc4d7f62a1aab6de53': {
      Check: 'task-happy',
      Group: 'trying-multi-dupes.fakepy[1]',
      ID: '73ad9b936fb3f3cc4d7f62a1aab6de53',
      Mode: 'healthiness',
      Output: 'nomad: http ok',
      Service: 'task-fake-py',
      Status: 'success',
      StatusCode: 200,
      Task: 'http.server',
      Timestamp: 1661787992,
    },
    '19421ef816ae0d3eeeb81697bce0e261': {
      Check: 'task-sad',
      Group: 'trying-multi-dupes.fakepy[1]',
      ID: '19421ef816ae0d3eeeb81697bce0e261',
      Mode: 'healthiness',
      Output:
        '<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN"\n        "http://www.w3.org/TR/html4/strict.dtd">\n<html>\n    <head>\n        <meta http-equiv="Content-Type" content="text/html;charset=utf-8">\n        <title>Error response</title>\n    </head>\n    <body>\n        <h1>Error response</h1>\n        <p>Error code: 404</p>\n        <p>Message: File not found.</p>\n        <p>Error code explanation: HTTPStatus.NOT_FOUND - Nothing matches the given URI.</p>\n    </body>\n</html>\n',
      Service: 'task-fake-py',
      Status: 'failure',
      StatusCode: 404,
      Task: 'http.server',
      Timestamp: 1661787965,
    },
    '784d40e33fa4c960355bbda79fbd20f0': {
      Check: 'tcp_probe',
      Group: 'trying-multi-dupes.fakepy[1]',
      ID: '784d40e33fa4c960355bbda79fbd20f0',
      Mode: 'readiness',
      Output: 'nomad: tcp ok',
      Service: 'web',
      Status: 'success',
      Task: 'http.server',
      Timestamp: 1661787995,
    },
  },
  isMigrating: false,
  wasPreempted: false,
  allocationTaskGroup: {
    Name: 'fakepy',
    Count: 3,
    Tasks: [
      {
        Name: 'http.server',
        Driver: 'raw_exec',
        Kind: '',
        Meta: null,
        Lifecycle: null,
        ReservedMemory: 300,
        ReservedMemoryMax: 0,
        ReservedCPU: 100,
        ReservedDisk: 0,
        ReservedEphemeralDisk: 300,
        Services: [
          {
            Name: 'task-fake-py',
            PortLabel: 'http',
            Tags: [],
            OnUpdate: 'require_healthy',
            Provider: 'nomad',
            Connect: null,
          },
          {
            Name: 'web',
            PortLabel: 'http',
            Tags: ['web', 'tcp', 'lol', 'lmao'],
            OnUpdate: 'require_healthy',
            Provider: 'nomad',
            Connect: null,
          },
          {
            Name: 'duper',
            PortLabel: 'http',
            Tags: ['web', 'tcp', 'lol', 'lmao'],
            OnUpdate: 'require_healthy',
            Provider: 'nomad',
            Connect: null,
          },
        ],
        VolumeMounts: null,
      },
    ],
    Services: [
      {
        Name: 'fake-py',
        PortLabel: 'http',
        Tags: [],
        OnUpdate: 'require_healthy',
        Provider: 'nomad',
        Connect: null,
      },
      {
        Name: 'duper',
        PortLabel: 'http',
        Tags: [],
        OnUpdate: 'require_healthy',
        Provider: 'nomad',
        Connect: null,
      },
    ],
    Volumes: [],
    Scaling: null,
    Meta: null,
    ReservedEphemeralDisk: 300,
  },
  states: [
    {
      Name: 'http.server',
      State: 'running',
      StartedAt: '2022-08-29T14:08:57.680Z',
      FinishedAt: null,
      Failed: false,
      Resources: {
        Cpu: 100,
        Memory: 300,
        MemoryMax: null,
        Disk: null,
        Iops: null,
        Networks: [],
        Ports: [],
      },
      Events: [
        {
          Type: 'Received',
          Signal: 0,
          ExitCode: 0,
          Time: '2022-08-29T14:08:57.592Z',
          TimeNanos: 865024,
          DisplayMessage: 'Task received by client',
        },
        {
          Type: 'Task Setup',
          Signal: 0,
          ExitCode: 0,
          Time: '2022-08-29T14:08:57.595Z',
          TimeNanos: 160064,
          DisplayMessage: 'Building Task Directory',
        },
        {
          Type: 'Started',
          Signal: 0,
          ExitCode: 0,
          Time: '2022-08-29T14:08:57.680Z',
          TimeNanos: 728064,
          DisplayMessage: 'Task started by client',
        },
        {
          Type: 'Alloc Unhealthy',
          Signal: 0,
          ExitCode: 0,
          Time: '2022-08-29T14:13:57.592Z',
          TimeNanos: 152064,
          DisplayMessage:
            'Task not running for min_healthy_time of 10s by healthy_deadline of 5m0s',
        },
      ],
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
