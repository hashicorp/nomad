/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { LinkTo } from '@ember/routing';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import willDestroy from '@ember/render-modifiers/modifiers/will-destroy';

export default class PluginSubnavComponent extends Component {
  @service keyboard;

  <template>
    <div
      data-test-subnav="plugins"
      class="tabs is-subnav"
      {{didInsert this.keyboard.registerNav type="subnav"}}
      {{willDestroy this.keyboard.unregisterSubnav}}
    >
      <ul>
        <li data-test-tab="overview">
          <LinkTo
            @route="storage.plugins.plugin.index"
            @model={{@plugin}}
            @activeClass="is-active"
          >
            Overview
          </LinkTo>
        </li>
        <li data-test-tab="allocations">
          <LinkTo
            @route="storage.plugins.plugin.allocations"
            @model={{@plugin}}
            @activeClass="is-active"
          >
            Allocations
          </LinkTo>
        </li>
      </ul>
    </div>
  </template>
}
