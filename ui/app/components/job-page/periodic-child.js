import AbstractJobPage from './abstract';
import { computed } from '@ember/object';

export default AbstractJobPage.extend({
  breadcrumbs: computed('job.{name,id}', 'job.parent.{name,id}', function() {
    const job = this.get('job');
    const parent = this.get('job.parent');

    return [
      { label: 'Jobs', args: ['jobs'] },
      {
        label: parent.get('name'),
        args: ['jobs.job', parent],
      },
      {
        label: job.get('trimmedName'),
        args: ['jobs.job', job],
      },
    ];
  }),
});
