/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, concat, fn, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { pageTitle } from 'ember-page-title';
import can from 'ember-can/helpers/can';
import eq from 'ember-truth-helpers/helpers/eq';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import ForbiddenMessage from 'nomad-ui/components/forbidden-message';
import VariablePaths from 'nomad-ui/components/variable-paths';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import {
  HdsButton,
  HdsDropdown,
  HdsPageHeader,
} from '@hashicorp/design-system-components/components';

<template>
  {{pageTitle "Variables: " @controller.absolutePath}}
  {{#each @controller.breadcrumbs as |crumb|}}
    <Breadcrumb @crumb={{crumb}} />
  {{/each}}
  <section class="section">
    <HdsPageHeader class="variable-title" as |PH|>
      <PH.Title>/{{@controller.absolutePath}}</PH.Title>
      <PH.Actions>
        {{#if @controller.namespaceOptions}}
          <HdsDropdown data-test-variable-namespace-filter as |dd|>
            <dd.ToggleButton
              @text="Namespace ({{@controller.namespaceSelection}})"
              @color="secondary"
            />
            {{#each @controller.namespaceOptions as |option|}}
              <dd.Radio
                name={{option.key}}
                {{on "change" (fn @controller.setNamespace option.key)}}
                checked={{eq @controller.namespaceSelection option.key}}
              >
                {{option.label}}
              </dd.Radio>
            {{/each}}
          </HdsDropdown>
        {{/if}}

        {{#if
          (can
            "write variable"
            path=(concat @controller.absolutePath "/")
            namespace=@controller.namespaceSelection
          )
        }}
          <div
            {{keyboardShortcut
              pattern=(array "n" "v")
              action=@controller.goToNewVariable
              label="Create Variable"
            }}
          >
            <HdsButton
              @text="Create Variable"
              @icon="plus"
              @route="variables.new"
              @query={{hash path=(concat @controller.absolutePath "/")}}
              data-test-create-var
            />
          </div>
        {{else}}
          <HdsButton
            @text="Create Variable"
            @icon="plus"
            data-test-create-var
            disabled
          />
        {{/if}}
      </PH.Actions>
    </HdsPageHeader>
    {{#if @controller.isForbidden}}
      <ForbiddenMessage />
    {{else}}
      {{#if @model.treeAtPath}}
        <VariablePaths @branch={{@model.treeAtPath}} />
      {{else}}
        <div class="empty-message">
          {{#if (eq @controller.namespaceSelection "*")}}
            <h3
              data-test-empty-variables-list-headline
              class="empty-message-headline"
            >
              Path /{{@controller.absolutePath}}
              contains no variables
            </h3>
            <p class="empty-message-body">
              To get started,
              <LinkTo
                @route="variables.new"
                @query={{hash path=(concat @controller.absolutePath "/")}}
              >create a new variable here</LinkTo>, or
              <LinkTo @route="variables">go back to the Variables root directory</LinkTo>.
            </p>
          {{else}}
            <h3
              data-test-no-matching-variables-list-headline
              class="empty-message-headline"
            >
              No Matches
            </h3>
            <p class="empty-message-body">
              No paths or variables match the namespace
              <strong>
                {{@controller.namespaceSelection}}
              </strong>
            </p>
          {{/if}}
        </div>
      {{/if}}
    {{/if}}
  </section>
</template>
