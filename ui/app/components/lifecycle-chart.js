import Component from '@ember/component';
import { computed } from '@ember/object';
import { sort } from '@ember/object/computed';

export default Component.extend({
  tagName: '',

  tasks: null,
  taskStates: null,

  lifecyclePhases: computed('tasks.@each.lifecycle', 'taskStates.@each.state', function() {
    const tasksOrStates = this.taskStates || this.tasks;
    const lifecycles = {
      prestarts: [],
      sidecars: [],
      mains: [],
    };

    tasksOrStates.forEach(taskOrState => {
      const task = taskOrState.task || taskOrState;
      lifecycles[`${task.lifecycleName}s`].push(taskOrState);
    });

    const phases = [];

    if (lifecycles.prestarts.length || lifecycles.sidecars.length) {
      phases.push({
        name: 'Prestart',
        isActive: lifecycles.prestarts.some(state => state.state === 'running'),
      });
    }

    if (lifecycles.sidecars.length || lifecycles.mains.length) {
      phases.push({
        name: 'Main',
        isActive: lifecycles.mains.some(state => state.state === 'running'),
      });
    }

    return phases;
  }),

  sortedLifecycleTaskStates: sort('taskStates', function(a, b) {
    return getTaskSortPrefix(a.task).localeCompare(getTaskSortPrefix(b.task));
  }),

  sortedLifecycleTasks: sort('tasks', function(a, b) {
    return getTaskSortPrefix(a).localeCompare(getTaskSortPrefix(b));
  }),
});

function getTaskSortPrefix(task) {
  return `${task.lifecycle ? (task.lifecycle.sidecar ? '1' : '0') : '2'}-${task.name}`;
}
