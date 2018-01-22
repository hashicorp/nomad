import Controller, { inject as controller } from '@ember/controller';
import { computed } from '@ember/object';
import queryParams from 'nomad-ui/utils/query-params';

export default Controller.extend({
  jobController: controller('jobs.job'),
  breadcrumbs: computed(
    'jobController.breadcrumbs.[]',
    'model.{name,job,taskGroupName,shortId}',
    'model.job.name',
    'model.job.namespace.name',
    function() {
      const jobBreadcrumbs = this.get('jobController.breadcrumbs');
      if (typeof jobBreadcrumbs[1].label === 'undefined') {
        jobBreadcrumbs[1] = {
          label: this.get('model.job.name'),
          params: [
            'jobs.job',
            this.get('model.job'),
            queryParams({
              jobNamespace: this.get('model.job.namespace.name'),
            }),
          ],
        };
      }
      return jobBreadcrumbs.concat([
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
