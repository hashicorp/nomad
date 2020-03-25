import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { filterBy, mapBy, or, sort } from '@ember/object/computed';
import generateExecUrl from 'nomad-ui/utils/generate-exec-url';
import openExecUrl from 'nomad-ui/utils/open-exec-url';

export default Component.extend({
  router: service(),

  isOpen: or('clickedOpen', 'currentRouteIsThisTaskGroup'),

  currentRouteIsThisTaskGroup: computed('router.currentRoute', function() {
    const route = this.router.currentRoute;

    if (route.name.includes('task-group')) {
      const taskGroupRoute = route.parent;
      const execRoute = taskGroupRoute.parent;

      return (
        execRoute.params.job_name === this.taskGroup.job.name &&
        taskGroupRoute.params.task_group_name === this.taskGroup.name
      );
    } else {
      return false;
    }
  }),

  hasPendingAllocations: computed('taskGroup.allocations.@each.clientStatus', function() {
    return this.taskGroup.allocations.any(allocation => allocation.clientStatus === 'pending');
  }),

  allocationTaskStatesRecordArrays: mapBy('taskGroup.allocations', 'states'),
  allocationTaskStates: computed('allocationTaskStatesRecordArrays.[]', function() {
    const flattenRecordArrays = (accumulator, recordArray) =>
      accumulator.concat(recordArray.toArray());
    return this.allocationTaskStatesRecordArrays.reduce(flattenRecordArrays, []);
  }),

  activeTaskStates: filterBy('allocationTaskStates', 'isActive'),

  activeTasks: mapBy('activeTaskStates', 'task'),
  activeTaskGroups: mapBy('activeTasks', 'taskGroup'),

  tasksWithRunningStates: computed(
    'taskGroup.name',
    'activeTaskStates.@each.name',
    'activeTasks.@each.name',
    'activeTaskGroups.@each.name',
    function() {
      const activeTaskStateNames = this.activeTaskStates
        .filter(taskState => {
          return taskState.task && taskState.task.taskGroup.name === this.taskGroup.name;
        })
        .mapBy('name');

      return this.taskGroup.tasks.filter(task => activeTaskStateNames.includes(task.name));
    }
  ),

  taskSorting: Object.freeze(['name']),
  sortedTasks: sort('tasksWithRunningStates', 'taskSorting'),

  clickedOpen: false,

  actions: {
    toggleOpen() {
      this.toggleProperty('clickedOpen');
    },

    openInNewWindow(job, taskGroup, task) {
      let url = generateExecUrl(this.router, {
        job: job.name,
        taskGroup: taskGroup.name,
        task: task.name,
      });

      openExecUrl(url);
    },
  },
});
