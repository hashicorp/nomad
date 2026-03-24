/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import eq from 'ember-truth-helpers/helpers/eq';
import formatTemplateLabel from 'nomad-ui/helpers/format-template-label';
import {
  HdsButton,
  HdsButtonSet,
  HdsFormRadioCardGroup,
  HdsLinkInline,
} from '@hashicorp/design-system-components/components';
import { on } from '@ember/modifier';
import { hash } from '@ember/helper';

<template>
  <section class="section">
    <header class="run-job-header">
      <h1 class="title is-3">Choose a template</h1>
      <p>Select a custom or default job template below. You will have an
        opportunity to modify and plan your job before it is submitted.</p>
    </header>
    {{#if (eq @controller.templates.length 0)}}
      <h3
        data-test-empty-templates-list-headline
        class="empty-message-headline"
      >
        You have no templates to choose from. Would you like to
        <HdsLinkInline
          @route="jobs.run.templates.new"
          data-test-create-inline
        >create</HdsLinkInline>
        one?
      </h3>
      <HdsButton
        class="button-group"
        @text="Back to editor"
        @route="jobs.run"
        @icon="arrow-left"
        data-test-cancel
      />
    {{else}}
      <main class="radio-group" data-test-template-list>
        <HdsFormRadioCardGroup as |G|>
          <G.Legend>Select a Template</G.Legend>
          {{#each @controller.templates as |card|}}
            <G.RadioCard
              class="form-container"
              @layout="fixed"
              @maxWidth="30%"
              @checked={{eq card.id @controller.selectedTemplate}}
              id={{card.id}}
              data-test-template-card={{formatTemplateLabel card.path}}
              {{on "change" @controller.onChange}}
              as |R|
            >
              <R.Label data-test-template-label>{{formatTemplateLabel
                  card.path
                }}</R.Label>
              <R.Description
                data-test-template-description
              >{{card.items.description}}</R.Description>
            </G.RadioCard>
          {{/each}}
        </HdsFormRadioCardGroup>
      </main>
      <footer class="buttonset">
        <HdsButtonSet class="button-group">
          <HdsButton
            @text="Apply"
            @route="jobs.run"
            @query={{hash template=@controller.selectedTemplate}}
            disabled={{eq @controller.selectedTemplate null}}
            data-test-apply
          />
          <HdsButton
            @text="Cancel"
            @route="jobs.run"
            @color="secondary"
            data-test-cancel
          />
        </HdsButtonSet>
        <HdsButtonSet class="button-group align-right">
          <HdsButton
            @text="Manage"
            @color="tertiary"
            @icon="edit"
            @route="jobs.run.templates.manage"
            data-test-manage-button
          />
          <HdsButton
            @text="Create New Template"
            @color="tertiary"
            @icon="file-plus"
            @route="jobs.run.templates.new"
            data-test-create-new-button
          />
        </HdsButtonSet>
      </footer>
    {{/if}}
  </section>
</template>
