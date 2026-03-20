/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { LinkTo } from '@ember/routing';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import willDestroy from '@ember/render-modifiers/modifiers/will-destroy';

export default class ClientSubnav extends Component {
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
            @route="clients.client.index"
            @model={{@client}}
            @activeClass="is-active"
          >
            Overview
          </LinkTo>
        </li>
        <li>
          <LinkTo
            @route="clients.client.monitor"
            @model={{@client}}
            @activeClass="is-active"
          >
            Monitor
          </LinkTo>
        </li>
      </ul>
    </div>
  </template>
}
