import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

export default class AllocationServiceSidebarComponent extends Component {
  @service store;

  get isSideBarOpen() {
    return !!this.args.service;
  }
  keyCommands = [
    {
      label: 'Close Evaluations Sidebar',
      pattern: ['Escape'],
      action: () => this.args.fns.closeSidebar(),
    },
  ];

  get service() {
    return this.store.query('service-fragment', { refID: this.args.serviceID });
  }

  get address() {
    const port = this.args.allocation?.allocatedResources?.ports?.findBy(
      'label',
      this.args.service.portLabel
    );
    if (port) {
      return `${port.hostIp}:${port.value}`;
    } else {
      return null;
    }
  }

  get aggregateStatus() {
    return this.args.service?.mostRecentChecks?.any(
      (check) => check.Status === 'failure'
    )
      ? 'Unhealthy'
      : 'Healthy';
  }
}
