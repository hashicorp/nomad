import { computed } from '@ember/object';
import DistributionBar from './distribution-bar';
import classic from 'ember-classic-decorator';

@classic
export default class ClientStatusBar extends DistributionBar {
  layoutName = 'components/distribution-bar';

  'data-test-client-status-bar' = true;
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
      },
      {
        label: 'Starting',
        value: starting.length,
        className: 'starting',
        layers: 2,
      },
      {
        label: 'Running',
        value: running.length,
        className: 'running',
      },
      {
        label: 'Complete',
        value: complete.length,
        className: 'complete',
      },
      {
        label: 'Degraded',
        value: degraded.length,
        className: 'degraded',
      },
      {
        label: 'Failed',
        value: failed.length,
        className: 'failed',
      },
      {
        label: 'Lost',
        value: lost.length,
        className: 'lost',
      },
      {
        label: 'Not Scheduled',
        value: notScheduled.length,
        className: 'not-scheduled',
      },
    ];
  }
}
