import { computed } from '@ember/object';
import DistributionBar from './distribution-bar';
import classic from 'ember-classic-decorator';
import { countBy } from 'lodash';

@classic
export default class ClientStatusBar extends DistributionBar {
  layoutName = 'components/distribution-bar';

  'data-test-client-status-bar' = true;
  allocationContainer = null;
  nodes = null;
  totalNodes = null;

  @computed('nodes')
  get data() {
    const statuses = {
      queued: 0,
      'not scheduled': this.totalNodes - this.nodes.length,
      starting: 0,
      running: 0,
      complete: 0,
      degraded: 0,
      failed: 0,
      lost: 0,
    };
    for (const node of this.nodes) {
      const concatenatedAllocationStatuses = [].concat(...Object.values(node));
      console.log(concatenatedAllocationStatuses);
      // there is a bug that counts nodes multiple times in this part of the loop
      for (const status of concatenatedAllocationStatuses) {
        const val = str => str;
        const statusCount = countBy(concatenatedAllocationStatuses, val);
        if (Object.keys(statusCount).length === 1) {
          if (statusCount.running > 0) {
            statuses.running++;
          }
          if (statusCount.failed > 0) {
            statuses.failed++;
          }
          if (statusCount.lost > 0) {
            statuses.lost++;
          }
          if (statusCount.complete > 0) {
            statuses.complete++;
          }
        } else if (Object.keys(statusCount).length !== 1 && !!statusCount.running) {
          if (!!statusCount.failed || !!statusCount.lost) {
            statuses.degraded++;
          }
        } else if (Object.keys(statusCount).length !== 1 && !!statusCount.pending) {
          statuses.starting++;
        } else {
          statuses.queued++;
        }
      }
    }

    console.log('statuses\n\n', statuses);
    return [
      { label: 'Not Scheduled', value: statuses['not scheduled'], className: 'not-scheduled' },
      { label: 'Queued', value: statuses.queued, className: 'queued' },
      {
        label: 'Starting',
        value: statuses.starting,
        className: 'starting',
        layers: 2,
      },
      { label: 'Running', value: statuses.running, className: 'running' },
      {
        label: 'Complete',
        value: statuses.complete,
        className: 'complete',
      },
      { label: 'Degraded', value: statuses.degraded, className: 'degraded' },
      { label: 'Failed', value: statuses.failed, className: 'failed' },
      { label: 'Lost', value: statuses.lost, className: 'lost' },
    ];
  }
}
