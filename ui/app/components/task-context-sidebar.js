// @ts-check
import Component from '@glimmer/component';

export default class TaskContextSidebarComponent extends Component {
  get isSideBarOpen() {
    return !!this.args.task;
  }

  keyCommands = [
    {
      label: 'Close Task Logs Sidebar',
      pattern: ['Escape'],
      action: () => this.args.fns.closeSidebar(),
    },
  ];
}
