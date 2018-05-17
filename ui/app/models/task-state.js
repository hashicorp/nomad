import { none } from '@ember/object/computed';
import { computed } from '@ember/object';
import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragment, fragmentOwner, fragmentArray } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  name: attr('string'),
  state: attr('string'),
  startedAt: attr('date'),
  finishedAt: attr('date'),
  failed: attr('boolean'),

  isActive: none('finishedAt'),

  allocation: fragmentOwner(),
  task: computed('allocation.taskGroup.tasks.[]', function() {
    const tasks = this.get('allocation.taskGroup.tasks');
    return tasks && tasks.findBy('name', this.get('name'));
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
