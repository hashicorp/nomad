/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array } from '@ember/helper';
import { on } from '@ember/modifier';
import { pageTitle } from 'ember-page-title';
import eq from 'ember-truth-helpers/helpers/eq';
import VariableForm from 'nomad-ui/components/variable-form';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import {
  HdsBreadcrumb,
  HdsBreadcrumbItem,
  HdsFormToggleField,
  HdsPageHeader,
} from '@hashicorp/design-system-components/components';

<template>
  {{pageTitle "Edit Variable"}}
  <HdsPageHeader class="variable-title" as |PH|>
    <PH.Title>Editing {{@model.path}}</PH.Title>
    <PH.IconTile @icon="file-text" />
    <PH.Actions>
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
    </PH.Actions>
    <PH.Breadcrumb>
      <HdsBreadcrumb>
        <HdsBreadcrumbItem
          @text="Back"
          @route="variables.variable.index"
          @icon="chevron-left"
        />
      </HdsBreadcrumb>
    </PH.Breadcrumb>
  </HdsPageHeader>

  <VariableForm
    @model={{@model}}
    @existingVariables={{@controller.existingVariables}}
    @view={{@controller.view}}
  />
</template>
