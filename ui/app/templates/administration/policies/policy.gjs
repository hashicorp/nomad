/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { pageTitle } from 'ember-page-title';
import can from 'ember-can/helpers/can';
import perform from 'ember-concurrency/helpers/perform';
import momentFromNow from 'ember-moment/helpers/moment-from-now';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import CopyButton from 'nomad-ui/components/copy-button';
import ListTable from 'nomad-ui/components/list-table';
import PolicyEditor from 'nomad-ui/components/policy-editor';
import Tooltip from 'nomad-ui/components/tooltip';
import TwoStepButton from 'nomad-ui/components/two-step-button';
import {
  HdsButton,
  HdsIcon,
  HdsPageHeader,
} from '@hashicorp/design-system-components/components';

<template>
  <Breadcrumb
    @crumb={{hash
      label=@controller.policy.name
      args=(array "administration.policies.policy" @controller.policy.name)
    }}
  />
  {{pageTitle "Policy"}}

  <section class="section">
    <HdsPageHeader as |PH|>
      <PH.Title data-test-title>{{@controller.policy.name}}</PH.Title>
      {{#if (can "destroy policy")}}
        <PH.Actions>
          <TwoStepButton
            data-test-delete-policy
            @alignRight={{true}}
            @idleText="Delete Policy"
            @cancelText="Cancel"
            @confirmText="Yes, Delete Policy"
            @confirmationMessage="Are you sure?"
            @awaitingConfirmation={{@controller.deletePolicy.isRunning}}
            @disabled={{@controller.deletePolicy.isRunning}}
            @onConfirm={{perform @controller.deletePolicy}}
          />
        </PH.Actions>
      {{/if}}
    </HdsPageHeader>
    <PolicyEditor @policy={{@controller.policy}} />

    {{#if (can "list token")}}
      <hr />

      <h2 class="title">
        Tokens
      </h2>

      {{#if (can "write token")}}
        <div class="token-operations">
          <div class="boxed-section">
            <div class="boxed-section-head">
              <h3>Create a Test Token</h3>
            </div>
            <div class="boxed-section-body">
              <p class="is-info">Create a test token that expires in 10 minutes
                for testing purposes.</p>
              <label>
                <button
                  type="button"
                  class="button is-info is-outlined create-test-token"
                  data-test-create-test-token
                  {{on "click" (perform @controller.createTestToken)}}
                >Create Test Token</button>
              </label>
            </div>
          </div>
          <div class="boxed-section">
            <div class="boxed-section-head">
              <h3>Create Tokens from the Nomad CLI</h3>
            </div>
            <div class="boxed-section-body">
              <p>When you're ready to create more tokens, you can do so via the
                <a
                  class="external-link"
                  href="https://developer.hashicorp.com/nomad/docs/commands"
                  target="_blank"
                  rel="noopener noreferrer"
                >Nomad CLI
                  <HdsIcon @name="external-link" @isInline={{true}} /></a>
                with the following:
                <pre>
                  <code>{{@controller.newTokenString}}</code>
                  <CopyButton
                    data-test-copy-button
                    @clipboardText={{@controller.newTokenString}}
                    @compact={{true}}
                  />
                </pre>
              </p>
            </div>
          </div>
        </div>
      {{/if}}

      {{#if @controller.tokens.length}}
        <ListTable
          @source={{@controller.tokens}}
          @class="no-mobile-condense"
          as |t|
        >
          <t.head>
            <th>Name</th>
            <th>Created</th>
            <th>Expires</th>
            {{#if (can "destroy token")}}
              <th>Delete</th>
            {{/if}}
          </t.head>
          <t.body as |row|>
            <tr data-test-policy-token-row>
              <td data-test-token-name={{row.model.name}}>
                <Tooltip @text={{row.model.id}}>
                  {{row.model.name}}
                </Tooltip>
              </td>
              <td>
                {{momentFromNow row.model.createTime interval=1000}}
              </td>
              <td>
                {{#if row.model.expirationTime}}
                  <Tooltip @text={{row.model.expirationTime}}>
                    <span
                      data-test-token-expiration-time
                      class="{{if row.model.isExpired 'has-text-danger'}}"
                    >{{momentFromNow
                        row.model.expirationTime
                        interval=1000
                      }}</span>
                  </Tooltip>
                {{else}}
                  <span>Never</span>
                {{/if}}
              </td>
              {{#if (can "destroy token")}}
                <td class="is-200px">
                  <HdsButton
                    @text="Delete Token"
                    @color="critical"
                    data-test-delete-token-button
                    {{on "click" (perform @controller.deleteToken row.model)}}
                  />
                </td>
              {{/if}}
            </tr>
          </t.body>
        </ListTable>
      {{else}}
        <div class="empty-message">
          <h3
            data-test-empty-policies-list-headline
            class="empty-message-headline"
          >
            No Tokens
          </h3>
          <p class="empty-message-body">
            No tokens are using this policy.
          </p>
        </div>
      {{/if}}
    {{/if}}

  </section>
</template>
