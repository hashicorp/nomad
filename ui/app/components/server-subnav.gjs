/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { LinkTo } from '@ember/routing';
import { didInsert, willDestroy } from '@ember/render-modifiers';

export default class ServerSubnav extends Component {
  @service keyboard;

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
            @route="servers.server.index"
            @model={{@server}}
            @activeClass="is-active"
          >
            Overview
          </LinkTo>
        </li>
        <li>
          <LinkTo
            @route="servers.server.monitor"
            @model={{@server}}
            @activeClass="is-active"
          >
            Monitor
          </LinkTo>
        </li>
      </ul>
    </div>
  </template>
}
