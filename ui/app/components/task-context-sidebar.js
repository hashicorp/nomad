// @ts-check
import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

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

  @tracked wide = false;
  @action toggleWide() {
    this.wide = !this.wide;
  }
}
