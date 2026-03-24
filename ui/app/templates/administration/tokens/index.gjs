/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, fn, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { findBy } from '@nullvoxpopuli/ember-composable-helpers';
import can from 'ember-can/helpers/can';
import perform from 'ember-concurrency/helpers/perform';
import eq from 'ember-truth-helpers/helpers/eq';
import momentFromNow from 'ember-moment/helpers/moment-from-now';
import Tooltip from 'nomad-ui/components/tooltip';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import {
  HdsButton,
  HdsTable,
  HdsTag,
} from '@hashicorp/design-system-components/components';

<template>
  <section class="section">
    <header class="acl-explainer">
      <p>
        ACL Tokens are associated with one or more policies or roles to grant
        specific capabilities. Users can use these to sign into, and operate,
        Nomad with the permissions laid out in their policies.
      </p>
      <div>
        {{#if (can "write token")}}
          <HdsButton
            @text="Create Token"
            @icon="plus"
            @route="administration.tokens.new"
            {{keyboardShortcut
              pattern=(array "n" "t")
              action=@controller.goToNewToken
              label="Create Token"
            }}
            data-test-create-token
          />
        {{else}}
          <HdsButton
            @text="Create Token"
            @icon="plus"
            disabled
            data-test-disabled-create-token
          />
        {{/if}}
      </div>
    </header>

    {{#if @model.tokens.length}}
      <HdsTable
        @caption="A list of tokens for this cluster"
        class="acl-table"
        @model={{@model.tokens}}
        @columns={{array
          (hash key="name" label="Name" isSortable=true)
          (hash key="type" label="Type" isSortable=true)
          (hash key="createTime" label="Created" isSortable=true)
          (hash key="expirationTime" label="Expires" isSortable=true)
          (hash key="roles" label="Roles")
          (hash key="policies" label="Policies")
          (hash key="delete" label="Delete")
        }}
        @sortBy="name"
      >
        <:body as |B|>
          <B.Tr
            {{keyboardShortcut
              enumerated=true
              action=(fn @controller.openToken B.data)
            }}
            data-test-token-row
          >
            <B.Td data-test-token-name={{B.data.name}}>
              {{#if (eq B.data.id @controller.selfToken.id)}}
                <strong>{{B.data.name}}</strong>
              {{else}}
                <LinkTo
                  @route="administration.tokens.token"
                  @model={{B.data.id}}
                >
                  {{B.data.name}}
                </LinkTo>
              {{/if}}
            </B.Td>
            <B.Td data-test-token-type={{B.data.type}}>{{B.data.type}}</B.Td>
            <B.Td>{{momentFromNow B.data.createTime interval=1000}}</B.Td>
            <B.Td>
              {{#if B.data.expirationTime}}
                <Tooltip @text={{B.data.expirationTime}}>
                  <span
                    data-test-token-expiration-time
                    class="{{if B.data.isExpired 'has-text-danger'}}"
                  >{{momentFromNow B.data.expirationTime interval=1000}}</span>
                </Tooltip>
              {{else}}
                <span data-test-token-expiration-time>Never</span>
              {{/if}}
            </B.Td>

            <B.Td data-test-token-roles>
              <div class="tag-group">
                {{#each B.data.roles as |role|}}
                  {{#if role.name}}
                    <HdsTag
                      @color="primary"
                      @text={{role.name}}
                      @route="administration.roles.role"
                      @model={{role.id}}
                    />
                  {{/if}}
                {{else}}
                  {{#if (eq B.data.type "management")}}
                    Management Access
                  {{else}}
                    No Roles
                  {{/if}}
                {{/each}}
              </div>
            </B.Td>

            <B.Td data-test-token-policies>
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
                  {{#if (eq B.data.type "management")}}
                    Management Access
                  {{else}}
                    No Policies
                  {{/if}}
                {{/each}}
              </div>
            </B.Td>

            {{#if (can "destroy token")}}
              <B.Td data-test-delete-token>
                {{#if (eq B.data.id @controller.selfToken.id)}}
                  <Tooltip
                    @text="Can't delete your own token"
                    @isFullText={{true}}
                  >
                    <HdsButton
                      @text="Delete"
                      disabled
                      @size="small"
                      @color="critical"
                    />
                  </Tooltip>
                {{else}}
                  <HdsButton
                    @text="Delete"
                    @size="small"
                    @color="critical"
                    {{on "click" (perform @controller.deleteToken B.data)}}
                  />
                {{/if}}
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
          No Tokens
        </h3>
        <p class="empty-message-body">
          Get started by
          <LinkTo @route="administration.policies.new">creating a new policy</LinkTo>
        </p>
      </div>
    {{/if}}
  </section>
</template>
