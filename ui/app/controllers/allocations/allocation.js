import Controller from '@ember/controller';
import { computed } from '@ember/object';

export default Controller.extend({
  breadcrumbs: computed('model.{job,taskGroupName,shortId}', 'model.job.name', function() {
    return [
      {
        label: 'Jobs',
        args: ['jobs'],
      },
      {
        label: this.get('model.job.name'),
        args: ['jobs.job', this.get('model.job')],
      },
      {
        label: this.get('model.taskGroupName'),
        args: ['jobs.job.task-group', this.get('model.job'), this.get('model.taskGroupName')],
      },
      {
        label: this.get('model.shortId'),
        args: ['allocations.allocation', this.get('model')],
      },
    ];
  }),
});
