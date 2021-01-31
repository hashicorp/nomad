import { computed } from '@ember/object';
import DistributionBar from './distribution-bar';

export default class AllocationStatusBar extends DistributionBar {
  layoutName = 'components/distribution-bar';

  allocationContainer = null;

  'data-test-allocation-status-bar' = true;

  @computed(
    'allocationContainer.{queuedAllocs,completeAllocs,failedAllocs,runningAllocs,startingAllocs}'
  )
  get data() {
    if (!this.allocationContainer) {
      return [];
    }

    const allocs = this.allocationContainer.getProperties(
      'queuedAllocs',
      'completeAllocs',
      'failedAllocs',
      'runningAllocs',
      'startingAllocs',
      'lostAllocs'
    );
    return [
      { label: 'Queued', value: allocs.queuedAllocs, className: 'queued' },
      {
        label: 'Starting',
        value: allocs.startingAllocs,
        className: 'starting',
        layers: 2,
      },
      { label: 'Running', value: allocs.runningAllocs, className: 'running' },
      {
        label: 'Complete',
        value: allocs.completeAllocs,
        className: 'complete',
      },
      { label: 'Failed', value: allocs.failedAllocs, className: 'failed' },
      { label: 'Lost', value: allocs.lostAllocs, className: 'lost' },
    ];
  }
}
