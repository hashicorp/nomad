/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { fn } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import { on } from '@ember/modifier';
import { service } from '@ember/service';
import { not } from 'ember-truth-helpers';
import conditionallyCapitalize from 'nomad-ui/helpers/conditionally-capitalize';

export default class ForbiddenMessage extends Component {
  @service token;
  @service store;
  @service router;

  forbiddenOriginPath = null;

  constructor() {
    super(...arguments);

    const currentURL = this.router.currentURL;
    if (
      !this.forbiddenOriginPath &&
      currentURL &&
      currentURL !== '/settings/tokens'
    ) {
      this.forbiddenOriginPath = currentURL;
    }

    if (currentURL && currentURL !== '/settings/tokens') {
      if (!this.token.postExpiryPath) {
        this.token.postExpiryPath = currentURL;
      }
      if (!this.token.forbiddenReturnPath) {
        this.token.forbiddenReturnPath = currentURL;
      }
    }
  }

  get authMethods() {
    return this.store.findAll('auth-method');
  }

  <template>
    <div data-test-error class="empty-message" ...attributes>
      <h3 data-test-error-title class="empty-message-headline">Not Authorized</h3>
      <p data-test-error-message class="empty-message-body">
        {{#if this.token.secret}}
          You currently lack the
          {{#if @permission}}
            <code>{{@permission}}</code>
          {{else}}
            required
          {{/if}}
          <LinkTo
            data-test-permission-link
            @route="settings.tokens"
            {{on
              "click"
              (fn (mut this.token.postExpiryPath) this.router.currentURL)
            }}
          >permission</LinkTo>
          for this resource.<br />
          Contact your administrator if this is an error.
        {{else}}
          {{#if this.authMethods}}
            Sign in with
            {{#each this.authMethods as |authMethod|}}
              <LinkTo @route="settings.tokens">{{authMethod.name}}</LinkTo>,
            {{/each}}
            or
          {{/if}}

          {{conditionallyCapitalize "provide" (not this.authMethods.length)}}
          a
          <LinkTo @route="settings.tokens">token</LinkTo>
          with the
          {{#if @permission}}
            <code>{{@permission}}</code>
          {{else}}
            requisite
          {{/if}}
          permission to view this.
        {{/if}}
      </p>

      {{#unless this.token.secret}}
        <p class="empty-message-body">
          If you have signed in via the Nomad CLI, authenticate with:
          <div class="terminal-container">
            <pre class="terminal"><span
                class="prompt"
              >$</span> nomad ui -authenticate</pre>
          </div>
        </p>
      {{/unless}}
    </div>
  </template>
}
