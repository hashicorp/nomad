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
    const formattedNodes = this.nodes.map(node => {
      const [[_, allocs]] = Object.entries(node);
      return allocs.map(alloc => alloc.clientStatus);
    });
    for (const node of formattedNodes) {
      const statusCount = countBy(node, status => status);
      const hasOnly1Status = Object.keys(statusCount).length === 1;

      if (hasOnly1Status) {
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
      } else if (!hasOnly1Status && !!statusCount.running) {
        if (!!statusCount.failed || !!statusCount.lost) {
          statuses.degraded++;
        } else if (statusCount.pending) {
          statuses.starting++;
        }
      } else {
        // if no allocations then queued -- job registered, hasn't been assigned clients to run -- no allocations
        // may only have this state for a few milliseconds
        statuses.queued++;
      }
    }

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
