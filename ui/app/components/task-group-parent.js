import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { or } from '@ember/object/computed';

export default Component.extend({
  router: service(),

  isOpen: or('clickedOpen', 'currentRouteIsThisTaskGroup'),

  currentRouteIsThisTaskGroup: computed('router.currentRoute', function() {
    const route = this.router.currentRoute;

    if (route.name === 'exec.task-group') {
      const execRoute = route.parent;

      return (
        execRoute.params.job_name === this.taskGroup.job.name &&
        route.params.task_group_name === this.taskGroup.name
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
  },
});
