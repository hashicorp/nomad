import { computed } from '@ember/object';
import DistributionBar from './distribution-bar';
import classic from 'ember-classic-decorator';

@classic
export default class JobClientStatusBar extends DistributionBar {
  layoutName = 'components/distribution-bar';

  'data-test-job-client-status-bar' = true;
  job = null;
  jobClientStatus = null;

  @computed('job.namespace', 'jobClientStatus.byStatus')
  get data() {
    const {
      queued,
      starting,
      running,
      complete,
      degraded,
      failed,
      lost,
      notScheduled,
    } = this.jobClientStatus.byStatus;

    return [
      {
        label: 'Queued',
        value: queued.length,
        className: 'queued',
        legendLink: {
          queryParams: {
            status: JSON.stringify(['queued']),
            namespace: this.job.namespace.get('id'),
          },
        },
      },
      {
        label: 'Starting',
        value: starting.length,
        className: 'starting',
        legendLink: {
          queryParams: {
            status: JSON.stringify(['starting']),
            namespace: this.job.namespace.get('id'),
          },
        },
        layers: 2,
      },
      {
        label: 'Running',
        value: running.length,
        className: 'running',
        legendLink: {
          queryParams: {
            status: JSON.stringify(['running']),
            namespace: this.job.namespace.get('id'),
          },
        },
      },
      {
        label: 'Complete',
        value: complete.length,
        className: 'complete',
        legendLink: {
          queryParams: {
            status: JSON.stringify(['complete']),
            namespace: this.job.namespace.get('id'),
          },
        },
      },
      {
        label: 'Degraded',
        value: degraded.length,
        className: 'degraded',
        legendLink: {
          queryParams: {
            status: JSON.stringify(['degraded']),
            namespace: this.job.namespace.get('id'),
          },
        },
        help: 'Some allocations for this job were not successfull or did not run.',
      },
      {
        label: 'Failed',
        value: failed.length,
        className: 'failed',
        legendLink: {
          queryParams: {
            status: JSON.stringify(['failed']),
            namespace: this.job.namespace.get('id'),
          },
        },
      },
      {
        label: 'Lost',
        value: lost.length,
        className: 'lost',
        legendLink: {
          queryParams: {
            status: JSON.stringify(['lost']),
            namespace: this.job.namespace.get('id'),
          },
        },
      },
      {
        label: 'Not Scheduled',
        value: notScheduled.length,
        className: 'not-scheduled',
        legendLink: {
          queryParams: {
            status: JSON.stringify(['notScheduled']),
            namespace: this.job.namespace.get('id'),
          },
        },
        help: 'No allocations for this job were scheduled into these clients.',
      },
    ];
  }
}
