import Component from '@ember/component';
import { computed } from '@ember/object';
import { sort } from '@ember/object/computed';

export default Component.extend({
  tagName: '',

  tasks: null,
  taskStates: null,

  lifecyclePhases: computed('tasks.@each.lifecycle', 'taskStates.@each.state', function() {
    const tasksOrStates = this.taskStates || this.tasks;
    const prestarts = [],
      sidecars = [],
      mains = [];

    tasksOrStates.forEach(taskOrState => {
      const lifecycle = taskOrState.task ? taskOrState.task.lifecycle : taskOrState.lifecycle;

      if (lifecycle) {
        if (lifecycle.sidecar) {
          sidecars.push(taskOrState);
        } else {
          prestarts.push(taskOrState);
        }
      } else {
        mains.push(taskOrState);
      }
    });

    const phases = [];

    if (prestarts.length || sidecars.length) {
      phases.push({
        name: 'Prestart',
        isActive: prestarts.some(state => state.state === 'running'),
      });
    }

    if (sidecars.length || mains.length) {
      phases.push({
        name: 'Main',
        isActive: mains.some(state => state.state === 'running'),
      });
    }

    return phases;
  }),

  sortedLifecycleTaskStates: sort('taskStates', function(a, b) {
    // FIXME sorts prestart, sidecar, main, secondary by name, correct?
    return getTaskSortPrefix(a.task).localeCompare(getTaskSortPrefix(b.task));
  }),

  sortedLifecycleTasks: sort('tasks', function(a, b) {
    // FIXME same
    return getTaskSortPrefix(a).localeCompare(getTaskSortPrefix(b));
  }),
});

function getTaskSortPrefix(task) {
  return `${task.lifecycle ? (task.lifecycle.sidecar ? '1' : '0') : '2'}-${task.name}`;
}
