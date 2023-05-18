import Component from '@glimmer/component';

export default class JobStatusFailedOrLostComponent extends Component {
  get shouldLinkToAllocations() {
    return this.args.allocs.length;
  }
}
