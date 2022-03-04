import { computed } from '@ember/object';
import DistributionBar from './distribution-bar';

export default class AllocationStatusBar extends DistributionBar {
  layoutName = 'components/distribution-bar';

  allocationContainer = null;
  job = null;

  'data-test-allocation-status-bar' = true;

  generateLegendLink(job, status) {
    if (!job || status === 'queued') return null;

    return {
      queryParams: {
        status: JSON.stringify([status]),
        namespace: job.belongsTo('namespace').id(),
      },
    };
  }

  @computed(
    'allocationContainer.{queuedAllocs,completeAllocs,failedAllocs,runningAllocs,startingAllocs}',
    'job.namespace'
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
      {
        label: 'Queued',
        value: allocs.queuedAllocs,
        className: 'queued',
        legendLink: this.generateLegendLink(this.job, 'queued'),
      },
      {
        label: 'Starting',
        value: allocs.startingAllocs,
        className: 'starting',
        layers: 2,
        legendLink: this.generateLegendLink(this.job, 'starting'),
      },
      {
        label: 'Running',
        value: allocs.runningAllocs,
        className: 'running',
        legendLink: this.generateLegendLink(this.job, 'running'),
      },
      {
        label: 'Complete',
        value: allocs.completeAllocs,
        className: 'complete',
        legendLink: this.generateLegendLink(this.job, 'complete'),
      },
      {
        label: 'Failed',
        value: allocs.failedAllocs,
        className: 'failed',
        legendLink: this.generateLegendLink(this.job, 'failed'),
      },
      {
        label: 'Lost',
        value: allocs.lostAllocs,
        className: 'lost',
        legendLink: this.generateLegendLink(this.job, 'lost'),
      },
    ];
  }
}
