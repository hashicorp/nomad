// @ts-check
import Component from '@glimmer/component';
import { alias } from '@ember/object/computed';
import matchGlob from 'nomad-ui/utils/match-glob';
import { inject as service } from '@ember/service';

export default class JobStatusPanelSteadyComponent extends Component {
  @service can;
  @alias('args.job') job;

  // Build note: allocTypes order matters! We will fill up to 100% of totalAllocs in this order.
  allocTypes = [
    'running',
    'pending',
    'failed',
    // 'unknown',
    'lost',
    // 'queued',
    // 'complete',
    'unplaced',
  ].map((type) => {
    return {
      label: type,
    };
  });

  get allocBlocks() {
    let availableSlotsToFill = this.totalAllocs;
    // Only fill up to 100% of totalAllocs. Once we've filled up, we can stop counting.
    let allocationsOfShowableType = this.allocTypes.reduce((blocks, type) => {
      const jobAllocsOfType = this.args.job.allocations
        .sortBy('jobVersion') // Try counting from latest deployment's allocs and work backwards if needed
        .reverse()
        .filterBy('clientStatus', type.label);
      if (availableSlotsToFill > 0) {
        blocks[type.label] = {
          healthy: {
            nonCanary: Array(
              Math.min(availableSlotsToFill, jobAllocsOfType.length)
            )
              .fill()
              .map((_, i) => {
                return jobAllocsOfType[i];
              }),
          },
        };
        availableSlotsToFill -= blocks[type.label].healthy.nonCanary.length;
      } else {
        blocks[type.label] = { healthy: { nonCanary: [] } };
      }
      return blocks;
    }, {});
    if (availableSlotsToFill > 0) {
      allocationsOfShowableType['unplaced'] = {
        healthy: {
          nonCanary: Array(availableSlotsToFill)
            .fill()
            .map(() => {
              return { clientStatus: 'unplaced' };
            }),
        },
      };
    }
    return allocationsOfShowableType;
  }

  get nodes() {
    return this.args.nodes;
  }

  get totalAllocs() {
    if (this.args.job.type === 'service') {
      return this.args.job.taskGroups.reduce((sum, tg) => sum + tg.count, 0);
    } else if (this.args.job.type === 'system') {
      if (this.totalAllocsCanBeKnown) {
        // Filter nodes by the datacenters defined in the job.
        const filteredNodes = this.nodes.filter((n) => {
          return this.args.job.datacenters.find((dc) => {
            return !!matchGlob(dc, n.datacenter);
          });
        });

        return filteredNodes.length;
      } else {
        // You don't have node read access, so do the best you can: uniqBy node IDs on the allocs you can see.
        return this.args.job.allocations.uniqBy('nodeID').length;
      }
    } else {
      return this.args.job.count; // TODO: this is probably not the correct totalAllocs count for any type.
    }
  }

  get totalAllocsCanBeKnown() {
    // System jobs don't have a group.count, so we depend on reading the overal number of nodes.
    // If the user lacks "read client" capabilities, this becomes impossible.
    if (this.args.job.type === 'system') {
      return this.can.can('read client');
    } else {
      return true;
    }
  }

  get versions() {
    return Object.values(this.allocBlocks)
      .flatMap((allocType) => Object.values(allocType))
      .flatMap((allocHealth) => Object.values(allocHealth))
      .flatMap((allocCanary) => Object.values(allocCanary))
      .map((a) => (!isNaN(a?.jobVersion) ? a.jobVersion : 'pending')) // "starting" allocs, and possibly others, do not yet have a jobVersion
      .reduce(
        (result, item) => ({
          ...result,
          [item]: [...(result[item] || []), item],
        }),
        []
      );
  }

  get rescheduledAllocs() {
    return this.job.allocations.filter(
      (a) => a.jobVersion === this.job.version && a.hasBeenRescheduled
    );
  }

  get restartedAllocs() {
    return this.job.allocations.filter(
      (a) => a.jobVersion === this.job.version && a.hasBeenRestarted
    );
  }

  get supportsRescheduling() {
    return this.job.type !== 'system';
  }
}
