// @ts-check
import { alias } from '@ember/object/computed';
import { action } from '@ember/object';
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';

export default class TaskContextSidebarComponent extends Component {
  get isSideBarOpen() {
    console.log('is it open?', this.args.task);
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
