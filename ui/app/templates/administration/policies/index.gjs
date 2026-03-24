/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, fn } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import can from 'ember-can/helpers/can';
import perform from 'ember-concurrency/helpers/perform';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import {
  HdsButton,
  HdsTable,
} from '@hashicorp/design-system-components/components';

<template>
  <section class="section">
    <header class="acl-explainer">
      <p>
        ACL Policies are sets of rules defining the capabilities granted to
        adhering tokens. You can create, modify, and delete them here.
      </p>
      <div>
        {{#if (can "write policy")}}
          <HdsButton
            @text="Create Policy"
            @icon="plus"
            @route="administration.policies.new"
            {{keyboardShortcut
              pattern=(array "n" "p")
              action=@controller.goToNewPolicy
              label="Create Policy"
            }}
            data-test-create-policy
          />
        {{else}}
          <HdsButton
            @text="Create Policy"
            @icon="plus"
            disabled
            data-test-disabled-create-policy
          />
        {{/if}}
      </div>
    </header>

    {{#if @controller.policies.length}}
      <HdsTable
        @caption="A list of policies for this cluster"
        class="acl-table"
        @model={{@controller.policies}}
        @columns={{@controller.columns}}
        @sortBy="name"
      >
        <:body as |B|>
          <B.Tr
            {{keyboardShortcut
              enumerated=true
              action=(fn @controller.openPolicy B.data)
            }}
            data-test-policy-row
          >
            <B.Td>
              <LinkTo
                data-test-policy-name={{B.data.name}}
                @route="administration.policies.policy"
                @model={{B.data.name}}
              >{{B.data.name}}</LinkTo>
            </B.Td>
            <B.Td>{{B.data.description}}</B.Td>
            {{#if (can "list token")}}
              <B.Td>
                <span
                  data-test-policy-total-tokens
                >{{B.data.tokens.length}}</span>
                {{#if B.data.expiredTokens.length}}
                  <span
                    data-test-policy-expired-tokens
                    class="number-expired"
                  >({{B.data.expiredTokens.length}}
                    expired)</span>
                {{/if}}
              </B.Td>
            {{/if}}
            {{#if (can "destroy policy")}}
              <B.Td>
                <HdsButton
                  @text="Delete"
                  @size="small"
                  @color="critical"
                  {{on "click" (perform @controller.deletePolicy B.data)}}
                  data-test-delete-policy
                />
              </B.Td>
            {{/if}}
          </B.Tr>
        </:body>
      </HdsTable>
    {{else}}
      <div class="empty-message">
        <h3
          data-test-empty-policies-list-headline
          class="empty-message-headline"
        >
          No Policies
        </h3>
        <p class="empty-message-body">
          Get started by
          <LinkTo @route="administration.policies.new">creating a new policy</LinkTo>
        </p>
      </div>
    {{/if}}
  </section>
</template>
