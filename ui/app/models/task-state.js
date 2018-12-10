import { computed } from '@ember/object';
import { alias, none, and } from '@ember/object/computed';
import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragment, fragmentOwner, fragmentArray } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  allocation: fragmentOwner(),

  name: attr('string'),
  state: attr('string'),
  startedAt: attr('date'),
  finishedAt: attr('date'),
  failed: attr('boolean'),

  isActive: none('finishedAt'),
  isRunning: and('isActive', 'allocation.isRunning'),

  task: computed('allocation.taskGroup.tasks.[]', function() {
    const tasks = this.get('allocation.taskGroup.tasks');
    return tasks && tasks.findBy('name', this.get('name'));
  }),

  driver: alias('task.driver'),

  // TaskState represents a task running on a node, so in addition to knowing the
  // driver via the task, the health of the driver is also known via the node
  driverStatus: computed('task.driver', 'allocation.node.drivers.[]', function() {
    const nodeDrivers = this.get('allocation.node.drivers') || [];
    return nodeDrivers.findBy('name', this.get('task.driver'));
  }),

  resources: fragment('resources'),
  events: fragmentArray('task-event'),

  stateClass: computed('state', function() {
    const classMap = {
      pending: 'is-pending',
      running: 'is-primary',
      finished: 'is-complete',
      failed: 'is-error',
    };

    return classMap[this.get('state')] || 'is-dark';
  }),
});
