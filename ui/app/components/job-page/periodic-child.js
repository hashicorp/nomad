import AbstractJobPage from './abstract';
import { computed } from '@ember/object';
import classic from 'ember-classic-decorator';

@classic
export default class PeriodicChild extends AbstractJobPage {
  @computed('job.{name,id}', 'job.parent.{name,id}')
  get breadcrumbs() {
    const job = this.job;
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
  }
}
