/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { pageTitle } from 'ember-page-title';
import { Input } from '@ember/component';
import cannot from 'ember-can/helpers/cannot';
import perform from 'ember-concurrency/helpers/perform';
import SingleSelectDropdown from 'nomad-ui/components/single-select-dropdown';
import TwoStepButton from 'nomad-ui/components/two-step-button';
import VariableFormJobTemplateEditor from 'nomad-ui/components/variable-form/job-template-editor';
import {
  HdsButton,
  HdsButtonSet,
  HdsModal,
} from '@hashicorp/design-system-components/components';

<template>
  {{pageTitle "Edit template"}}
  <section class="section">
    <header class="run-job-header">
      <div>
        <h1 class="title is-3">Edit template</h1>
      </div>
      <TwoStepButton
        data-test-delete
        @alignRight={{true}}
        @idleText="Delete"
        @cancelText="Cancel"
        @confirmText="Yes, Delete Template"
        @inlineText={{true}}
        @confirmationMessage="Are you sure?"
        @awaitingConfirmation={{@controller.deleteTemplate.isRunning}}
        @disabled={{cannot "destroy variable" namespace="*"}}
        @onConfirm={{perform @controller.deleteTemplate}}
      />
    </header>
    <form class="new-job-template" autocomplete="off">
      <div
        class={{if
          @controller.system.shouldShowNamespaces
          "input-dropdown-row"
        }}
      >
        <label>
          <span>
            Template name
          </span>
          <Input
            @type="text"
            @value={{@model.path}}
            class="input path-input"
            disabled
            data-test-template-name
          />
        </label>
        {{#if @controller.system.shouldShowNamespaces}}
          <label>
            <span>
              Namespace
            </span>
            <SingleSelectDropdown
              data-test-namespace-facet
              @label="Namespace"
              @selection={{@model.namespace}}
              @options={{array
                (hash key=@model.namespace label=@model.namespace)
              }}
              @disabled={{true}}
            />
          </label>
        {{/if}}
      </div>
      <VariableFormJobTemplateEditor
        @keyValues={{@model.keyValues}}
        @updateKeyValue={{@controller.updateKeyValue}}
      />
      <footer class="button-group">
        <HdsButton
          @text="Edit"
          {{on "click" @controller.save}}
          data-test-edit-template
        />
        <HdsButton
          @text="Cancel"
          @route="jobs.run.templates"
          @color="critical"
          data-test-cancel-template
        />
      </footer>
    </form>
  </section>
  {{#if @controller.formModalActive}}
    <HdsModal
      id="form-modal"
      @onClose={{@controller.toggleModal}}
      @color="critical"
      data-test-confirmation-modal
      as |M|
    >
      <M.Header>
        Confirm
      </M.Header>
      <M.Body>
        Are you sure you want to delete this template?
      </M.Body>
      <M.Footer as |F|>
        <HdsButtonSet>
          <HdsButton
            type="submit"
            @text="Confirm"
            {{on "click" @controller.deleteTemplateAndClose}}
            data-test-delete-template
          />
          <HdsButton
            type="button"
            @text="Cancel"
            @color="secondary"
            {{on "click" F.close}}
          />
        </HdsButtonSet>
      </M.Footer>
    </HdsModal>
  {{/if}}
</template>
