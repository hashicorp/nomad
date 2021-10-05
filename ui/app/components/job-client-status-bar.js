import { computed } from '@ember/object';
import DistributionBar from './distribution-bar';
import classic from 'ember-classic-decorator';

@classic
export default class JobClientStatusBar extends DistributionBar {
  layoutName = 'components/distribution-bar';

  'data-test-job-client-status-bar' = true;
  jobClientStatus = null;
  onSliceClick() {}

  @computed('jobClientStatus.byStatus')
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
        queryParams: { status: JSON.stringify(['queued']) },
      },
      {
        label: 'Starting',
        value: starting.length,
        className: 'starting',
        queryParams: { status: JSON.stringify(['starting']) },
        layers: 2,
      },
      {
        label: 'Running',
        value: running.length,
        className: 'running',
        queryParams: { status: JSON.stringify(['running']) },
      },
      {
        label: 'Complete',
        value: complete.length,
        className: 'complete',
        queryParams: { status: JSON.stringify(['complete']) },
      },
      {
        label: 'Degraded',
        value: degraded.length,
        className: 'degraded',
        queryParams: { status: JSON.stringify(['degraded']) },
        help: 'Some allocations for this job were not successfull or did not run.',
      },
      {
        label: 'Failed',
        value: failed.length,
        className: 'failed',
        queryParams: { status: JSON.stringify(['failed']) },
      },
      {
        label: 'Lost',
        value: lost.length,
        className: 'lost',
        queryParams: { status: JSON.stringify(['lost']) },
      },
      {
        label: 'Not Scheduled',
        value: notScheduled.length,
        className: 'not-scheduled',
        queryParams: { status: JSON.stringify(['notScheduled']) },
        help: 'No allocations for this job were scheduled into these clients.',
      },
    ];
  }
}
