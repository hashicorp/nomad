import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';

export default Controller.extend({
  breadcrumbs: computed('model.job', function() {
    return [
      { label: 'Jobs', args: ['jobs'] },
      {
        label: this.get('model.job.name'),
        args: [
          'jobs.job',
          this.get('model.job.plainId'),
          qpBuilder({
            jobNamespace: this.get('model.job.namespace.name') || 'default',
          }),
        ],
      },
      {
        label: this.get('model.taskGroupName'),
        args: [
          'jobs.job.task-group',
          this.get('model.job'),
          this.get('model.taskGroupName'),
          qpBuilder({
            jobNamespace: this.get('model.namespace.name') || 'default',
          }),
        ],
      },
      {
        label: this.get('model.shortId'),
        args: ['allocations.allocation', this.get('model')],
      },
    ];
  }),
});
