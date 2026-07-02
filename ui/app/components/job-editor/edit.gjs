/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, fn } from '@ember/helper';
import { on } from '@ember/modifier';
import can from 'ember-can/helpers/can';
import { eq, not, or } from 'ember-truth-helpers';
import {
  HdsButton,
  HdsButtonSet,
  HdsFormToggleField,
} from '@hashicorp/design-system-components/components';
import Tooltip from 'nomad-ui/components/tooltip';
import codeMirror from 'nomad-ui/modifiers/code-mirror';
import keyboardShortcutModifier from 'nomad-ui/modifiers/keyboard-shortcut';

const performTask = (task) => {
  if (typeof task?.perform === 'function') {
    task.perform();
  }
};

export const JobEditorEdit = <template>
  <div class="boxed-section">
    <div class="boxed-section-head">
      Job Definition
      {{#if @data.cancelable}}
        <div class="pull-right" style="display: flex">
          <span class="header-toggle">
            <HdsFormToggleField
              {{keyboardShortcutModifier
                label="Toggle word wrap"
                action=@fns.onToggleWrap
                pattern=(array "w" "w")
                menuLevel=true
              }}
              checked={{@data.shouldWrap}}
              {{on "change" @fns.onToggleWrap}}
              as |F|
            >
              <F.Label>Word Wrap</F.Label>
            </HdsFormToggleField>
          </span>
          <Tooltip
            @condition={{unless @data.hasSpecification true false}}
            @isFullText={{true}}
            @text="A jobspec file was not submitted when this job was run. You can still view and edit the expanded JSON format."
          >
            <div
              class="job-definition-select
                {{unless @data.hasSpecification ' disabled'}}"
              data-test-select={{@data.view}}
            >
              <button
                class="button is-small is-borderless
                  {{if (eq @data.view 'job-spec') 'is-active'}}"
                type="button"
                {{on "click" (fn @fns.onSelect "job-spec")}}
              >
                Job Spec
              </button>
              <button
                class="button is-small is-borderless
                  {{if (eq @data.view 'full-definition') 'is-active'}}"
                type="button"
                {{on "click" (fn @fns.onSelect "full-definition")}}
                data-test-select-full
              >
                Full Definition
              </button>
            </div>
          </Tooltip>
          <button
            class="button is-light is-compact pull-right"
            {{on "click" @fns.onCancel}}
            type="button"
            data-test-cancel-editing
          >
            Cancel
          </button>
        </div>
      {{/if}}
    </div>
    <div class="boxed-section-body is-full-bleed">
      <div
        data-test-editor
        {{codeMirror
          screenReaderLabel="Job definition"
          content=@data.job._newDefinition
          theme="hashi"
          onUpdate=@fns.onUpdate
          mode=(if (eq @data.format "json") "javascript" "ruby")
          lineWrapping=@data.shouldWrap
        }}
      ></div>
    </div>
    {{#if (or (eq @data.view "job-spec") @data.job.isNew)}}
      <div class="boxed-section" style="margin-top: 10px">
        <div class="boxed-section-head">
          {{#if @data.job.isNew}}
            HCL Variable Values
          {{else}}
            Edit HCL Variable Values
          {{/if}}
        </div>
        <div class="boxed-section-body is-full-bleed">
          <div
            class="json-viewer is-variable-editor"
            data-test-variable-editor
            {{codeMirror
              autofocus=false
              screenReaderLabel="HLC Variables for Job Spec"
              content=@data.job._newDefinitionVariables
              theme="hashi"
              onUpdate=@fns.onUpdate
              type="hclVariables"
              mode="ruby"
              lineWrapping=@data.shouldWrap
            }}
          ></div>
        </div>
      </div>
    {{/if}}
  </div>
  <HdsButtonSet class="is-associative buttonset sticky pull-left">
    <HdsButton
      {{on "click" (fn performTask @fns.onPlan)}}
      disabled={{or @fns.onPlan.isRunning (not @data.job._newDefinition)}}
      data-test-plan
      @text="Plan"
    />
    {{#if (can "write variable" path="nomad/job-templates/*" namespace="*")}}
      {{#if @data.job.isNew}}
        <HdsButton
          @text="Save as template"
          @color="secondary"
          @route="jobs.run.templates.new"
          {{on "click" @fns.onSaveAs}}
          data-test-save-as-template
        />
      {{/if}}
    {{/if}}
    <HdsButton
      @text="Save as .nomad.hcl"
      @color="secondary"
      {{on "click" @fns.onSaveFile}}
      disabled={{not @data.job._newDefinition}}
    />
  </HdsButtonSet>
</template>;

export default JobEditorEdit;
