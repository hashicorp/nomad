/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { concat } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import { pageTitle } from 'ember-page-title';
import can from 'ember-can/helpers/can';
import cannot from 'ember-can/helpers/cannot';
import perform from 'ember-concurrency/helpers/perform';
import eq from 'ember-truth-helpers/helpers/eq';
import not from 'ember-truth-helpers/helpers/not';
import or from 'ember-truth-helpers/helpers/or';
import formatTemplateLabel from 'nomad-ui/helpers/format-template-label';
import TwoStepButton from 'nomad-ui/components/two-step-button';
import {
  HdsButton,
  HdsButtonSet,
  HdsLinkInline,
  HdsTable,
} from '@hashicorp/design-system-components/components';

<template>
  {{pageTitle "Manage templates"}}
  <section class="section">
    <header class="run-job-header">
      <h1 class="title is-3">Manage Job Templates</h1>
      <p>Modify or Delete a job template from the list below. Default job
        templates cannot be removed.</p>
    </header>
    {{#if (eq @model.length 0)}}
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
        <HdsTable
          @model={{@controller.templates}}
          @columns={{@controller.columns}}
          @isFixedLayout={{true}}
        >
          <:body as |B|>
            <B.Tr>
              <B.Td>
                {{#if
                  (or
                    B.data.isDefaultJobTemplate
                    (not
                      (can
                        "write variable"
                        path="nomad/job-templates/*"
                        namespace="*"
                      )
                    )
                  )
                }}
                  {{formatTemplateLabel B.data.path}}
                {{else}}
                  <LinkTo
                    @route="jobs.run.templates.template"
                    @model={{concat B.data.path "@" B.data.namespace}}
                    data-test-edit-template={{B.data.path}}
                  >
                    {{formatTemplateLabel B.data.path}}
                  </LinkTo>
                {{/if}}
              </B.Td>
              <B.Td>{{B.data.namespace}}</B.Td>
              <B.Td>
                {{B.data.items.description}}
              </B.Td>
              <B.Td>
                {{#if B.data.isDefaultJobTemplate}}
                  <em>(Default Job - cannot be deleted)</em>
                {{else}}
                  <TwoStepButton
                    data-test-delete
                    @idleText="Delete"
                    @cancelText="Cancel"
                    @confirmText="Yes"
                    @inlineText={{true}}
                    @confirmationMessage="Are you sure?"
                    @awaitingConfirmation={{@controller.deleteTemplate.isRunning}}
                    @disabled={{cannot "destroy variable" namespace="*"}}
                    @onConfirm={{perform @controller.deleteTemplate B.data}}
                  />
                {{/if}}
              </B.Td>
            </B.Tr>
          </:body>
        </HdsTable>
      </main>
      <footer class="buttonset">
        <HdsButtonSet class="button-group">
          <HdsButton
            @text="Cancel"
            @color="secondary"
            @route="jobs.run.templates"
            data-test-done
          />
        </HdsButtonSet>
      </footer>
    {{/if}}
  </section>
</template>
