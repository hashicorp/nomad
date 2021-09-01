import { classNames, tagName } from '@ember-decorators/component';
import EmberObject from '@ember/object';
import Component from '@glimmer/component';

@tagName('tr')
@classNames('client-row', 'is-interactive')
export default class ClientRowComponent extends Component {
  get shouldDisplayAllocationSummary() {
    return this.status !== 'notScheduled';
  }
  get node() {
    return this.args.node.model;
  }

  get eldestCreateTime() {
    let eldest = null;
    for (const allocation of this.node.id) {
      if (!eldest || allocation.createTime < eldest) {
        eldest = allocation.createTime;
      }
    }
    return eldest;
  }

  get mostRecentModifyTime() {
    let mostRecent = null;
    for (const allocation of this.node.id) {
      if (!mostRecent || allocation.modifyTime > mostRecent) {
        mostRecent = allocation.createTime;
      }
    }
    return mostRecent;
  }

  get status() {
    return this.args.jobClientStatus.byNode[this.node.id];
  }

  get allocationContainer() {
    const statusSummary = {
      queuedAllocs: 0,
      completeAllocs: 0,
      failedAllocs: 0,
      runningAllocs: 0,
      startingAllocs: 0,
      lostAllocs: 0,
    };
    // query by allocations for job then group by node use the mapBy method
    if (this.status === 'notScheduled') return EmberObject.create(...statusSummary);

    const allocsByNodeID = {};
    this.args.allocations.forEach(a => {
      const nodeId = a.node.get('id');
      if (!allocsByNodeID[nodeId]) {
        allocsByNodeID[nodeId] = [];
      }
      allocsByNodeID[nodeId].push(a);
    });
    for (const allocation of allocsByNodeID[this.node.id]) {
      if (this.status === 'queued') {
        statusSummary.queuedAllocs = allocsByNodeID[this.node.id].length;
        break;
      } else if (this.status === 'starting') {
        statusSummary.startingAllocs = allocsByNodeID[this.node.id].length;
        break;
      } else if (this.status === 'notScheduled') {
        break;
      }
      const { clientStatus } = allocation;
      switch (clientStatus) {
        case 'running':
          statusSummary.runningAllocs++;
          break;
        case 'lost':
          statusSummary.lostAllocs++;
          break;
        case 'failed':
          statusSummary.failedAllocs++;
          break;
        case 'complete':
          statusSummary.completeAllocs++;
          break;
        case 'starting':
          statusSummary.startingAllocs++;
          break;
      }
    }
    const Allocations = EmberObject.extend({
      ...statusSummary,
    });
    return Allocations.create();
  }
}
