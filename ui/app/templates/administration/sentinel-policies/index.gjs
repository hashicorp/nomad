/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, fn } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import { pageTitle } from 'ember-page-title';
import can from 'ember-can/helpers/can';
import perform from 'ember-concurrency/helpers/perform';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import TwoStepButton from 'nomad-ui/components/two-step-button';
import {
  HdsButton,
  HdsLinkInline,
  HdsPageHeader,
  HdsTable,
} from '@hashicorp/design-system-components/components';

<template>
  {{pageTitle "Sentinel Policies"}}

  <section class="section">
    <HdsPageHeader class="variable-title" as |PH|>
      <PH.Title>Sentinel Policies</PH.Title>
      <PH.Description>
        Nomad integrates with
        <HdsLinkInline
          @icon="collections"
          @href="https://developer.hashicorp.com/nomad/docs/govern/sentinel"
        >HashiCorp Sentinel</HdsLinkInline>
        to allow operators to express policies as code and have those policies
        automatically enforced. This allows operators to define a "sandbox" and
        restrict actions to only those compliant with that policy.
      </PH.Description>
      <PH.Actions>
        {{#if (can "write sentinel-policy")}}
          <span
            {{keyboardShortcut
              pattern=(array "n" "p")
              action=@controller.goToNewPolicy
              label="Create Policy"
            }}
          >
            <HdsButton
              @text="Create from Scratch"
              @icon="plus"
              @route="administration.sentinel-policies.new"
              data-test-create-sentinel-policy
            />
          </span>
          <span
            {{keyboardShortcut
              pattern=(array "n" "t" "p")
              action=@controller.goToTemplateGallery
              label="Create Policy from Template"
            }}
          >
            <HdsButton
              @text="Create from Template"
              @icon="plus"
              @route="administration.sentinel-policies.gallery"
              data-test-create-sentinel-policy-from-template
            />
          </span>
        {{else}}
          <HdsButton
            @text="Create Policy"
            @icon="plus"
            disabled
            data-test-disabled-create-sentinel-policy
          />
        {{/if}}
      </PH.Actions>
    </HdsPageHeader>

    {{#if @model}}
      <HdsTable
        @caption="A list of policies for this cluster"
        class="acl-table"
        @model={{@model}}
        @columns={{@controller.columns}}
        @sortBy="name"
      >
        <:body as |B|>
          <B.Tr
            {{keyboardShortcut
              enumerated=true
              action=(fn @controller.openPolicy B.data)
            }}
            data-test-sentinel-policy-row
          >
            <B.Td>
              <LinkTo
                data-test-sentinel-policy-name={{B.data.name}}
                @route="administration.sentinel-policies.policy"
                @model={{B.data.name}}
              >{{B.data.name}}</LinkTo>
            </B.Td>
            <B.Td
              data-test-sentinel-policy-description
            >{{B.data.description}}</B.Td>
            <B.Td
              data-test-sentinel-policy-enforcement
            >{{B.data.enforcementLevel}}</B.Td>
            <B.Td data-test-sentinel-policy-scope>{{B.data.scope}}</B.Td>
            {{#if (can "destroy sentinel-policy")}}
              <B.Td>
                <TwoStepButton
                  data-test-delete-policy
                  @idleText="Delete"
                  @inlineText={{true}}
                  @cancelText="Cancel"
                  @confirmText="Yes, Delete Policy"
                  @confirmationMessage="Are you sure?"
                  @awaitingConfirmation={{@controller.deletePolicy.isRunning}}
                  @disabled={{@controller.deletePolicy.isRunning}}
                  @onConfirm={{perform @controller.deletePolicy B.data}}
                />
              </B.Td>
            {{/if}}
          </B.Tr>
        </:body>
      </HdsTable>
    {{else}}
      <div data-test-empty-sentinel-policy-list class="empty-message">
        <h3
          data-test-empty-sentinel-policy-list-headline
          class="empty-message-headline"
        >
          No Sentinel Policies
        </h3>
        <p class="empty-message-body">
          Get started by
          <LinkTo @route="administration.sentinel-policies.new">creating a
            policy from scratch</LinkTo>
          or by
          <LinkTo @route="administration.sentinel-policies.gallery">creating one
            from the policy gallery</LinkTo>.
        </p>
      </div>
    {{/if}}
  </section>
</template>
