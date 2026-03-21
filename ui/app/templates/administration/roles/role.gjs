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
import RoleEditor from 'nomad-ui/components/role-editor';
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
      label=@controller.role.name
      args=(array "administration.roles.role" @controller.role.id)
    }}
  />
  {{pageTitle "Role"}}

  <section class="section">
    <HdsPageHeader as |PH|>
      <PH.Title data-test-title>
        {{@controller.role.name}}
      </PH.Title>
      {{#if (can "destroy role")}}
        <PH.Actions>
          <TwoStepButton
            data-test-delete-role
            @alignRight={{true}}
            @idleText="Delete Role"
            @cancelText="Cancel"
            @confirmText="Yes, Delete Role"
            @confirmationMessage="Are you sure?"
            @awaitingConfirmation={{@controller.deleteRole.isRunning}}
            @disabled={{@controller.deleteRole.isRunning}}
            @onConfirm={{perform @controller.deleteRole}}
          />
        </PH.Actions>
      {{/if}}
    </HdsPageHeader>

    <RoleEditor @role={{@controller.role}} @policies={{@controller.policies}} />

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
                <HdsButton
                  @text="Create Test Token"
                  @color="secondary"
                  data-test-create-test-token
                  class="create-test-token"
                  {{on "click" (perform @controller.createTestToken)}}
                />
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
                  href="https://developer.hashicorp.com/nomad/commands"
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
          @class="tokens no-mobile-condense"
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
            <tr data-test-role-token-row>
              <td data-test-token-name>
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
          <h3 data-test-empty-role-list-headline class="empty-message-headline">
            No Tokens
          </h3>
          <p class="empty-message-body">
            No tokens are using this role.
          </p>
        </div>
      {{/if}}
    {{/if}}

  </section>
</template>
