import { classNames, tagName } from '@ember-decorators/component';
import EmberObject from '@ember/object';
import Component from '@glimmer/component';

@tagName('tr')
@classNames('client-row', 'is-interactive')
export default class ClientRowComponent extends Component {
  get id() {
    console.log('node arg', this.args.node);
    return Object.keys(this.args.node.model)[0];
  }

  get name() {
    return this.args.node.model[this.id][0].name;
  }

  get eldestCreateTime() {
    let eldest = null;
    for (const allocation of this.args.node.model[this.id]) {
      if (!eldest || allocation.createTime < eldest) {
        eldest = allocation.createTime;
      }
    }
    return eldest;
  }

  get mostRecentModifyTime() {
    let mostRecent = null;
    for (const allocation of this.args.node.model[this.id]) {
      if (!mostRecent || allocation.modifyTime > mostRecent) {
        mostRecent = allocation.createTime;
      }
    }
    return mostRecent;
  }

  get status() {
    return this.args.jobClientStatus.byNode[this.id];
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
    for (const allocation of this.args.node.model[this.id]) {
      const { clientStatus } = allocation;
      switch (clientStatus) {
        // add missing statuses
        case 'running':
          statusSummary.runningAllocs++;
          break;
        case 'lost':
          statusSummary.lostAllocs++;
          break;
        case 'failed':
          statusSummary.failedAllocs++;
          break;
        case 'completed':
          statusSummary.completeAllocs++;
          break;
      }
    }
    const Allocations = EmberObject.extend({
      ...statusSummary,
    });
    return Allocations.create();
  }
}
