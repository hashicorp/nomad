/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, fn, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import can from 'ember-can/helpers/can';
import perform from 'ember-concurrency/helpers/perform';
import eq from 'ember-truth-helpers/helpers/eq';
import or from 'ember-truth-helpers/helpers/or';
import CopyButton from 'nomad-ui/components/copy-button';
import JsonViewer from 'nomad-ui/components/json-viewer';
import TwoStepButton from 'nomad-ui/components/two-step-button';
import VariableFormRelatedEntities from 'nomad-ui/components/variable-form/related-entities';
import stringifyObject from 'nomad-ui/helpers/stringify-object';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import {
  HdsButton,
  HdsCopyButton,
  HdsFormToggleField,
  HdsIcon,
  HdsPageHeader,
  HdsTable,
} from '@hashicorp/design-system-components/components';
import autofocus from 'nomad-ui/modifiers/autofocus';

<template>
  <HdsPageHeader class="variable-title" as |PH|>
    <PH.Title>{{@model.path}}</PH.Title>
    <PH.IconTile @icon="file-text" />
    <PH.Actions>
      {{#unless @controller.isDeleting}}

        <HdsFormToggleField
          @value="enable"
          {{keyboardShortcut
            label="Toggle View (JSON/List)"
            pattern=(array "j")
            action=@controller.toggleView
          }}
          checked={{eq @controller.view "json"}}
          data-test-json-toggle
          {{on "change" @controller.toggleView}}
          as |F|
        >
          <F.Label>JSON</F.Label>
        </HdsFormToggleField>

        <div
          {{keyboardShortcut
            label="Copy Variable"
            pattern=(array "c" "v")
            action=@controller.copyVariable
          }}
        >
          <HdsCopyButton
            @text="Copy"
            @textToCopy={{stringifyObject @model.items}}
            @isIconOnly={{true}}
            class="copy-variable"
          />
        </div>

        {{#if
          (can "write variable" path=@model.path namespace=@model.namespace)
        }}
          <HdsButton
            @icon="edit"
            @text="Edit"
            @color="secondary"
            @route="variables.variable.edit"
            @model={{@model}}
            @query={{hash view=@controller.view}}
            data-test-edit-button
            {{autofocus}}
          />
        {{/if}}
      {{/unless}}

      {{#if
        (can "destroy variable" path=@model.path namespace=@model.namespace)
      }}
        <TwoStepButton
          data-test-delete-button
          @alignRight={{true}}
          @idleText="Delete"
          @cancelText="Cancel"
          @confirmText="Yes, delete"
          @confirmationMessage="Are you sure you want to delete this variable and all its items?"
          @awaitingConfirmation={{@controller.deleteVariableFile.isRunning}}
          @onConfirm={{perform @controller.deleteVariableFile}}
          @onPrompt={{@controller.onDeletePrompt}}
          @onCancel={{@controller.onDeleteCancel}}
        />
      {{/if}}
    </PH.Actions>
  </HdsPageHeader>

  {{#if @controller.shouldShowLinkedEntities}}
    <VariableFormRelatedEntities
      @job={{@model.pathLinkedEntities.job}}
      @group={{@model.pathLinkedEntities.group}}
      @task={{@model.pathLinkedEntities.task}}
      @namespace={{or @model.namespace "default"}}
    />
  {{/if}}

  {{#if (eq @controller.view "json")}}
    <div class="boxed-section">
      <div class="boxed-section-head">
        Key/Value Data
      </div>
      <div class="boxed-section-body is-full-bleed">
        <JsonViewer @json={{@model.items}} />
      </div>
    </div>
  {{else}}
    <HdsTable
      class="variable-items"
      @model={{@controller.sortedKeyValues}}
      @sortBy={{@controller.sortProperty}}
      @sortOrder={{if @controller.sortDescending "desc" "asc"}}
      @columns={{array
        (hash key="key" label="Key" isSortable=true width="200px")
        (hash key="value" label="Value" isSortable=true)
      }}
    >
      <:body as |B|>
        <B.Tr data-test-var={{B.data.key}}>
          <B.Td>{{B.data.key}}</B.Td>
          <B.Td class="value-cell">
            <div>
              <CopyButton @compact={{true}} @clipboardText={{B.data.value}} />
              <button
                class="show-hide-values button is-borderless is-compact"
                type="button"
                {{on "click" (fn @controller.toggleRowVisibility B.data)}}
                {{keyboardShortcut
                  label="Toggle Variable Visibility"
                  pattern=(array "v")
                  action=(fn @controller.toggleRowVisibility B.data)
                }}
              >
                <HdsIcon
                  @name={{if B.data.isVisible "eye" "eye-off"}}
                  @title={{if B.data.isVisible "Hide Value" "Show Value"}}
                  @isInline={{true}}
                />
              </button>

              {{#if B.data.isVisible}}
                <code>{{B.data.value}}</code>
              {{else}}
                ********
              {{/if}}
            </div>
          </B.Td>
        </B.Tr>
      </:body>
    </HdsTable>

  {{/if}}
</template>
