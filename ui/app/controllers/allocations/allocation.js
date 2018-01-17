import Controller, { inject as controller } from '@ember/controller';
import { computed } from '@ember/object';

export default Controller.extend({
  jobController: controller('jobs.job'),
  breadcrumbs: computed(
    'jobController.breadcrumbs.[]',
    'model.{name,job,taskGroupName,shortId}',
    function() {
      return this.get('jobController.breadcrumbs').concat([
        {
          label: this.get('model.taskGroupName'),
          params: ['jobs.job.task-group', this.get('model.job'), this.get('model.taskGroupName')],
        },
        {
          label: this.get('model.shortId'),
          params: ['allocations.allocation', this.get('model')],
        },
      ]);
    }
  ),
});
