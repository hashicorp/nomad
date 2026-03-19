/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { LinkTo } from '@ember/routing';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import willDestroy from '@ember/render-modifiers/modifiers/will-destroy';
import can from 'ember-can/helpers/can';

export default class AdministrationSubnav extends Component {
  @service keyboard;

  <template>
    <div
      class="tabs is-subnav"
      {{didInsert this.keyboard.registerNav type="subnav"}}
      {{willDestroy this.keyboard.unregisterSubnav}}
    >
      <ul>
        <li>
          <LinkTo @route="administration.index" @activeClass="is-active">
            Overview
          </LinkTo>
        </li>
        <li>
          <LinkTo @route="administration.tokens" @activeClass="is-active">
            Tokens
          </LinkTo>
        </li>
        <li>
          <LinkTo @route="administration.roles" @activeClass="is-active">
            Roles
          </LinkTo>
        </li>
        <li>
          <LinkTo @route="administration.policies" @activeClass="is-active">
            Policies
          </LinkTo>
        </li>
        <li>
          <LinkTo @route="administration.namespaces" @activeClass="is-active">
            Namespaces
          </LinkTo>
        </li>
        {{#if (can "list sentinel-policy")}}
          <li>
            <LinkTo
              @route="administration.sentinel-policies"
              @activeClass="is-active"
            >
              Sentinel Policies
            </LinkTo>
          </li>
        {{/if}}
      </ul>
    </div>
  </template>
}
