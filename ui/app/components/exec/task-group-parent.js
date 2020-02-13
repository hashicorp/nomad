import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { or } from '@ember/object/computed';

export default Component.extend({
  router: service(),

  // FIXME this shouldn’t auto-close when switching to another task group if it has been manually opened already
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

  clickedOpen: false,

  actions: {
    toggleOpen() {
      this.toggleProperty('clickedOpen');
    },

    openInNewWindow(job, taskGroup, task) {
      // FIXME any way to construct this path via the router?
      window.open(
        `/ui/exec/${job.name}/${taskGroup.name}/${task.name}`,
        '_blank',
        'width=973,height=490,location=1'
      );
    },
  },
});
