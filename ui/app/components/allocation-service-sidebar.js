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
}
