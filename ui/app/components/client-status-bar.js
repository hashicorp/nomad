import { computed } from '@ember/object';
import DistributionBar from './distribution-bar';
import classic from 'ember-classic-decorator';

@classic
export default class ClientStatusBar extends DistributionBar {
  layoutName = 'components/distribution-bar';

  'data-test-client-status-bar' = true;
  jobClientStatus = null;

  @computed('jobClientStatus')
  get data() {
    return [
      {
        label: 'Queued',
        value: this.jobClientStatus.byStatus.queued.length,
        className: 'queued',
      },
      {
        label: 'Starting',
        value: this.jobClientStatus.byStatus.starting.length,
        className: 'starting',
      },
      {
        label: 'Running',
        value: this.jobClientStatus.byStatus.running.length,
        className: 'running',
      },
      {
        label: 'Complete',
        value: this.jobClientStatus.byStatus.complete.length,
        className: 'complete',
      },
      {
        label: 'Degraded',
        value: this.jobClientStatus.byStatus.degraded.length,
        className: 'degraded',
      },
      {
        label: 'Failed',
        value: this.jobClientStatus.byStatus.failed.length,
        className: 'failed',
      },
      {
        label: 'Lost',
        value: this.jobClientStatus.byStatus.lost.length,
        className: 'lost',
      },
      {
        label: 'Not Scheduled',
        value: this.jobClientStatus.byStatus.notScheduled.length,
        className: 'not-scheduled',
      },
    ];
  }
}
