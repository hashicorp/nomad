/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

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

  narrowCommand = {
    label: 'Narrow Sidebar',
    pattern: ['ArrowRight', 'ArrowRight'],
    action: () => this.toggleWide(),
  };

  widenCommand = {
    label: 'Widen Sidebar',
    pattern: ['ArrowLeft', 'ArrowLeft'],
    action: () => this.toggleWide(),
  };

  @tracked wide = false;
  @action toggleWide() {
    this.wide = !this.wide;
  }
}
