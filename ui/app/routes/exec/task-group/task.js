import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';

export default Route.extend({
  store: service(),

  model({ task_name }) {
    const allocationQueryParam = this.paramsFor('exec').allocation;
    const taskGroupName = this.paramsFor('exec.task-group').task_group_name;

    return {
      allocationShortId: allocationQueryParam,
      taskName: task_name,
      taskGroupName,
    };
  },

  setupController(controller, { allocationShortId, taskGroupName, taskName }) {
    this.controllerFor('exec').send('setTaskProperties', {
      allocationShortId,
      taskName,
      taskGroupName,
    });

    this._super(...arguments);
  },
});
