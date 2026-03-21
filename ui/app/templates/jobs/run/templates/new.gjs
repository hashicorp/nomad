/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { on } from '@ember/modifier';
import { pageTitle } from 'ember-page-title';
import isEmpty from 'ember-truth-helpers/helpers/is-empty';
import { HdsButton } from '@hashicorp/design-system-components/components';
import { Input } from '@ember/component';
import autofocus from 'nomad-ui/modifiers/autofocus';
import VariableFormJobTemplateEditor from 'nomad-ui/components/variable-form/job-template-editor';
import SingleSelectDropdown from 'nomad-ui/components/single-select-dropdown';

<template>
  {{pageTitle "Create a custom template"}}
  <section class="section">
    <header class="run-job-header">
      <h1 class="title is-3">Create template</h1>
      <p>Provide a job spec that you or others can re-use later. Anytime it is
        applied to a new job, you will have the opportunity to modify it before
        that job is run.</p>
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
            @value={{@controller.templateName}}
            placeholder="your-template-name-here"
            class="input path-input
              {{if @controller.isDuplicateTemplate 'error'}}"
            {{autofocus}}
            data-test-template-name
          />
          {{#if @controller.isDuplicateTemplate}}
            <p class="help is-danger" data-test-duplicate-error>
              There is already a templated named
              {{@controller.templateName}}.
            </p>
          {{/if}}
          {{#if @controller.hasInvalidName}}
            <p class="help is-danger" data-test-invalid-name-error>
              Template name must contain only alphanumeric or "-", "_", "~", or
              "/" characters, and be fewer than 128 characters in length.
            </p>
          {{/if}}
        </label>
        {{#if @controller.system.shouldShowNamespaces}}
          <label>
            <span>
              Namespace
            </span>
            <SingleSelectDropdown
              data-test-namespace-facet
              @label="Namespace"
              @options={{@controller.namespaceOptions}}
              @selection={{@controller.templateNamespace}}
              @onSelect={{@controller.setTemplateNamespace}}
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
          @text="Save"
          disabled={{isEmpty @controller.templateName}}
          {{on "click" @controller.save}}
          data-test-save-template
        />
        <HdsButton
          @text="Cancel"
          @route="jobs.run"
          @color="secondary"
          data-test-cancel-template
        />
      </footer>
    </form>
  </section>
</template>
