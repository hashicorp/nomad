import Component from '@glimmer/component';

export default class AllocationServiceSidebarComponent extends Component {
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
}
