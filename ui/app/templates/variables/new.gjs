/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { pageTitle } from 'ember-page-title';
import eq from 'ember-truth-helpers/helpers/eq';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import VariableForm from 'nomad-ui/components/variable-form';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import {
  HdsFormToggleField,
  HdsPageHeader,
} from '@hashicorp/design-system-components/components';

<template>
  {{pageTitle "New Variable"}}
  <Breadcrumb @crumb={{hash label="New" args=(array "variables.new")}} />

  <section class="section">
    <HdsPageHeader class="variable-title" as |PH|>
      <PH.Title>Create a Variable</PH.Title>
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
    </HdsPageHeader>

    <VariableForm
      @model={{@model}}
      @path={{@controller.path}}
      @existingVariables={{@controller.existingVariables}}
      @view={{@controller.view}}
    />
  </section>
</template>
