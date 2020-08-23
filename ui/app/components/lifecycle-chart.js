import Component from '@ember/component';
import { computed } from '@ember/object';
import { sort } from '@ember/object/computed';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('')
export default class LifecycleChart extends Component {
  tasks = null;
  taskStates = null;

  @computed('tasks.@each.lifecycle', 'taskStates.@each.state')
  get lifecyclePhases() {
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
  }

  @sort('taskStates', function(a, b) {
    return getTaskSortPrefix(a.task).localeCompare(getTaskSortPrefix(b.task));
  })
  sortedLifecycleTaskStates;

  @sort('tasks', function(a, b) {
    return getTaskSortPrefix(a).localeCompare(getTaskSortPrefix(b));
  })
  sortedLifecycleTasks;
}

const lifecycleNameSortPrefix = {
  prestart: 0,
  sidecar: 1,
  main: 2,
};

function getTaskSortPrefix(task) {
  // Prestarts first, then sidecars, then mains
  return `${lifecycleNameSortPrefix[task.lifecycleName]}-${task.name}`;
}
