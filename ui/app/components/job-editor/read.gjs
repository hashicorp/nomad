/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, fn } from '@ember/helper';
import { on } from '@ember/modifier';
import { and, eq } from 'ember-truth-helpers';
import { HdsFormToggleField } from '@hashicorp/design-system-components/components';
import Tooltip from 'nomad-ui/components/tooltip';
import codeMirror from 'nomad-ui/modifiers/code-mirror';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';

export const JobEditorRead = <template>
  <div class="boxed-section">
    <div class="boxed-section-head">
      Job Definition
      <div class="pull-right" style="display: flex">
        <span class="header-toggle">
          <HdsFormToggleField
            {{keyboardShortcut
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
          class="button is-light is-compact"
          type="button"
          {{on "click" @fns.onEdit}}
          data-test-edit-job
        >
          Edit
        </button>

      </div>
    </div>
    <div class="boxed-section-body is-full-bleed">
      {{#if (eq @data.view "job-spec")}}
        <div
          data-test-job-spec-view
          {{codeMirror
            content=@data.definition
            mode=(if (eq @data.format "json") "javascript" "ruby")
            readOnly=true
            screenReaderLabel="Job specification"
            theme="hashi-read-only"
            lineWrapping=@data.shouldWrap
          }}
        />
      {{else}}
        <div
          data-test-json-viewer
          {{codeMirror
            content=@data.definition
            theme="hashi-read-only"
            readOnly=true
            screenReaderLabel="JSON Viewer"
            lineWrapping=@data.shouldWrap
          }}
        />
      {{/if}}
    </div>
    {{#if (and (eq @data.view "job-spec") @data.hasVariables)}}
      <div class="boxed-section" style="margin-top: 10px">
        <div class="boxed-section-head">
          HCL Variable Values
        </div>
        <div class="boxed-section-body is-full-bleed">
          <div
            class="json-viewer is-variable-editor"
            data-test-variable-editor
            {{codeMirror
              screenReaderLabel="HLC Variables for Job Spec"
              content=@data.definitionVariables
              mode="ruby"
              theme="hashi-read-only"
              readOnly=true
              lineWrapping=@data.shouldWrap
            }}
          />
        </div>
      </div>
    {{/if}}
  </div>
</template>;

export default JobEditorRead;
