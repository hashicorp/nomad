/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { LinkTo } from '@ember/routing';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import willDestroy from '@ember/render-modifiers/modifiers/will-destroy';

export default class StorageSubnav extends Component {
  @service keyboard;

  <template>
    <div
      class="tabs is-subnav"
      {{didInsert this.keyboard.registerNav type="subnav"}}
      {{willDestroy this.keyboard.unregisterSubnav}}
    >
      <ul>
        <li data-test-tab="overview">
          <LinkTo @route="storage.index" @activeClass="is-active">
            Overview
          </LinkTo>
        </li>
        <li data-test-tab="plugins">
          <LinkTo @route="storage.plugins.index" @activeClass="is-active">
            CSI Plugins
          </LinkTo>
        </li>
      </ul>
    </div>
  </template>
}
