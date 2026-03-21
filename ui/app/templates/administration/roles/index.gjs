/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, fn } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { findBy } from '@nullvoxpopuli/ember-composable-helpers';
import can from 'ember-can/helpers/can';
import perform from 'ember-concurrency/helpers/perform';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import {
  HdsButton,
  HdsTable,
  HdsTag,
} from '@hashicorp/design-system-components/components';
import Tooltip from 'nomad-ui/components/tooltip';

<template>
  <section class="section">
    <header class="acl-explainer">
      <p>
        ACL Roles group one or more Policies into higher-level sets of
        permissions. A user token can have any number of roles or policies.
      </p>
      <div>
        {{#if (can "write role")}}
          <HdsButton
            @text="Create Role"
            @icon="plus"
            @route="administration.roles.new"
            {{keyboardShortcut
              pattern=(array "n" "r")
              action=@controller.goToNewRole
              label="Create Role"
            }}
            data-test-create-role
          />
        {{else}}
          <HdsButton
            @text="Create Role"
            @icon="plus"
            disabled
            data-test-disabled-create-role
          />
        {{/if}}
      </div>
    </header>

    {{#if @controller.roles.length}}
      <HdsTable
        @caption="A list of roles for this cluster"
        class="acl-table"
        @model={{@controller.roles}}
        @columns={{@controller.columns}}
        @sortBy="name"
      >
        <:body as |B|>
          <B.Tr
            {{keyboardShortcut
              enumerated=true
              action=(fn @controller.openRole B.data)
            }}
            data-test-role-row={{B.data.name}}
          >
            <B.Td data-test-role-name={{B.data.name}}>
              <LinkTo
                @route="administration.roles.role"
                @model={{B.data.id}}
              >{{B.data.name}}</LinkTo>
            </B.Td>
            <B.Td data-test-role-description>{{B.data.description}}</B.Td>

            {{#if (can "list token")}}
              <B.Td>
                <span
                  data-test-role-total-tokens
                >{{B.data.tokens.length}}</span>
                {{#if B.data.expiredTokens.length}}
                  <span
                    data-test-role-expired-tokens
                    class="number-expired"
                  >({{B.data.expiredTokens.length}}
                    expired)</span>
                {{/if}}
              </B.Td>
            {{/if}}

            {{#if (can "list policy")}}
              <B.Td data-test-role-policies>
                <div class="tag-group">
                  {{#each B.data.policyNames as |policyName|}}
                    {{#let
                      (findBy "name" policyName @model.policies)
                      as |policy|
                    }}
                      {{#if policy}}
                        <HdsTag
                          @color="primary"
                          @text={{policy.name}}
                          @route="administration.policies.policy"
                          @model={{policy.name}}
                        />
                      {{else}}
                        <Tooltip @text="This policy has been deleted">
                          <HdsTag @text={{policyName}} />
                        </Tooltip>
                      {{/if}}
                    {{/let}}
                  {{else}}
                    No Policies
                  {{/each}}
                </div>
              </B.Td>
            {{/if}}

            {{#if (can "destroy role")}}
              <B.Td>
                <HdsButton
                  @text="Delete"
                  @size="small"
                  @color="critical"
                  {{on "click" (perform @controller.deleteRole B.data)}}
                  data-test-delete-role
                />
              </B.Td>
            {{/if}}
          </B.Tr>
        </:body>
      </HdsTable>

    {{else}}
      <div class="empty-message">
        <h3 data-test-empty-role-list-headline class="empty-message-headline">
          No Roles
        </h3>
        <p class="empty-message-body">
          Get started by
          <LinkTo @route="administration.roles.new">creating a new role</LinkTo>
        </p>
      </div>
    {{/if}}
  </section>
</template>
