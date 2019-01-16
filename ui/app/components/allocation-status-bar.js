import { computed } from '@ember/object';
import DistributionBar from './distribution-bar';

export default DistributionBar.extend({
  layoutName: 'components/distribution-bar',

  allocationContainer: null,

  'data-test-allocation-status-bar': true,

  data: computed(
    'allocationContainer.{queuedAllocs,completeAllocs,failedAllocs,runningAllocs,startingAllocs}',
    function() {
      if (!this.get('allocationContainer')) {
        return [];
      }

      const allocs = this.get('allocationContainer').getProperties(
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
  ),
});
