import Component from '@ember/component';
import { computed } from '@ember/object';
import { mapBy, sort } from '@ember/object/computed';

export default Component.extend({
  tagName: '',

  stateTasks: mapBy('taskStates', 'task'),

  lifecyclePhases: computed('stateTasks.@each.lifecycle', 'taskStates.@each.state', function() {
    // FIXME OOC obvsy
    let tasksOrTaskStates = this.get('taskStates') || this.get('tasks');

    const lifecycleTaskStateLists = tasksOrTaskStates.reduce(
      (lists, taskOrTaskState) => {
        const lifecycle = taskOrTaskState.task
          ? taskOrTaskState.task.lifecycle
          : taskOrTaskState.lifecycle;

        if (lifecycle) {
          if (lifecycle.sidecar) {
            lists.sidecars.push(taskOrTaskState);
          } else {
            lists.prestarts.push(taskOrTaskState);
          }
        } else {
          lists.mains.push(taskOrTaskState);
        }

        return lists;
      },
      {
        prestarts: [],
        sidecars: [],
        mains: [],
      }
    );

    const phases = [];

    if (lifecycleTaskStateLists.prestarts.length || lifecycleTaskStateLists.sidecars.length) {
      phases.push({
        name: 'PreStart',
        isActive: lifecycleTaskStateLists.prestarts.some(state => state.state === 'running'),
      });
    }

    if (lifecycleTaskStateLists.sidecars.length || lifecycleTaskStateLists.mains.length) {
      phases.push({
        name: 'Main',
        isActive: lifecycleTaskStateLists.mains.some(state => state.state === 'running'),
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
