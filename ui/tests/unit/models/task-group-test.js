import { get } from '@ember/object';
import { moduleForModel, test } from 'ember-qunit';

const sum = (list, key) => list.reduce((sum, item) => sum + get(item, key), 0);

moduleForModel('task-group', 'Unit | Model | task-group', {
  needs: ['model:task', 'model:task-group-summary'],
});

test("should expose reserved resource stats as aggregates of each task's reserved resources", function(
  assert
) {
  const taskGroup = this.subject({
    name: 'group-example',
    tasks: [
      {
        name: 'task-one',
        driver: 'docker',
        reservedMemory: 512,
        reservedCPU: 500,
        reservedDisk: 1024,
      },
      {
        name: 'task-two',
        driver: 'docker',
        reservedMemory: 256,
        reservedCPU: 1000,
        reservedDisk: 512,
      },
      {
        name: 'task-three',
        driver: 'docker',
        reservedMemory: 1024,
        reservedCPU: 1500,
        reservedDisk: 4096,
      },
      {
        name: 'task-four',
        driver: 'docker',
        reservedMemory: 2048,
        reservedCPU: 500,
        reservedDisk: 128,
      },
    ],
  });

  assert.equal(
    taskGroup.get('reservedCPU'),
    sum(taskGroup.get('tasks'), 'reservedCPU'),
    'reservedCPU is an aggregate sum of task CPU reservations'
  );
  assert.equal(
    taskGroup.get('reservedMemory'),
    sum(taskGroup.get('tasks'), 'reservedMemory'),
    'reservedMemory is an aggregate sum of task memory reservations'
  );
  assert.equal(
    taskGroup.get('reservedDisk'),
    sum(taskGroup.get('tasks'), 'reservedDisk'),
    'reservedDisk is an aggregate sum of task disk reservations'
  );
});
