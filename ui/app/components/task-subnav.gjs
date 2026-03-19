/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { LinkTo } from '@ember/routing';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import willDestroy from '@ember/render-modifiers/modifiers/will-destroy';

export default class TaskSubnav extends Component {
  @service router;
  @service keyboard;

  get fsIsActive() {
    return this.router.currentRouteName === 'allocations.allocation.task.fs';
  }

  get fsRootIsActive() {
    return (
      this.router.currentRouteName === 'allocations.allocation.task.fs-root'
    );
  }

  get filesLinkActive() {
    return this.fsIsActive || this.fsRootIsActive;
  }

  get taskModels() {
    const task = this.args.task;
    if (!task) return [];
    return [task.allocation, task.name];
  }

  <template>
    <div
      class="tabs is-subnav"
      {{didInsert this.keyboard.registerNav type="subnav"}}
      {{willDestroy this.keyboard.unregisterSubnav}}
    >
      <ul>
        <li>
          <LinkTo
            @route="allocations.allocation.task.index"
            @models={{this.taskModels}}
            @activeClass="is-active"
          >
            Overview
          </LinkTo>
        </li>
        <li>
          <LinkTo
            @route="allocations.allocation.task.logs"
            @models={{this.taskModels}}
            @activeClass="is-active"
          >
            Logs
          </LinkTo>
        </li>
        <li>
          <LinkTo
            @route="allocations.allocation.task.fs-root"
            @models={{this.taskModels}}
            class={{if this.filesLinkActive "is-active"}}
          >
            Files
          </LinkTo>
        </li>
      </ul>
    </div>
  </template>
}
