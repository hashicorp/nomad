/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { LinkTo } from '@ember/routing';
import { didInsert, willDestroy } from '@ember/render-modifiers';

export default class AllocationSubnav extends Component {
  @service router;
  @service keyboard;

  get filesLinkActive() {
    return [
      'allocations.allocation.fs',
      'allocations.allocation.fs-root',
    ].includes(this.router.currentRouteName);
  }

  <template>
    <div
      class="tabs is-subnav"
      ...attributes
      {{didInsert this.keyboard.registerNav type="subnav"}}
      {{willDestroy this.keyboard.unregisterSubnav}}
    >
      <ul>
        <li>
          <LinkTo
            @route="allocations.allocation.index"
            @model={{@allocation}}
            @activeClass="is-active"
          >
            Overview
          </LinkTo>
        </li>
        <li>
          <LinkTo
            @route="allocations.allocation.fs-root"
            @model={{@allocation}}
            class={{if this.filesLinkActive "is-active"}}
          >
            Files
          </LinkTo>
        </li>
      </ul>
    </div>
  </template>
}
