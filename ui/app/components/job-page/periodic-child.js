import AbstractJobPage from './abstract';
import { computed } from '@ember/object';
import { inject as service } from '@ember/service';
import classic from 'ember-classic-decorator';
import jobClientStatus from 'nomad-ui/utils/properties/job-client-status';

@classic
export default class PeriodicChild extends AbstractJobPage {
  @service store;

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

  @jobClientStatus('nodes', 'job') jobClientStatus;

  get nodes() {
    return this.store.peekAll('node');
  }
}
