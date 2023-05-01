import Component from '@glimmer/component';

export default class JobStatusFailedOrLostComponent extends Component {
  get shouldLinkToAllocations() {
    return this.args.title !== 'Restarted' && this.args.allocs.length;
  }
}
