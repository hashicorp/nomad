/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { on } from '@ember/modifier';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import codeMirror from 'nomad-ui/modifiers/code-mirror';

export default class JobTemplateEditor extends Component {
  @tracked description;
  @tracked template;

  establishKeyValues = () => {
    this.description = this.args.keyValues?.find?.(
      (entry) => entry.key === 'description',
    )?.value;
    this.template = this.args.keyValues?.find?.(
      (entry) => entry.key === 'template',
    )?.value;
  };

  updateDescription = (event) => {
    this.args.updateKeyValue('description', event.target.value);
  };

  updateTemplate = (value) => {
    this.args.updateKeyValue('template', value);
  };

  <template>
    <span hidden {{didInsert this.establishKeyValues}}></span>
    <div>
      <label>
        <span>
          Description
        </span>
        <input
          type="text"
          value={{this.description}}
          {{on "input" this.updateDescription}}
          class="input value-input"
          data-test-template-description
        />
      </label>
    </div>
    <div>
      <label>
        <span>
          Job Template
        </span>
        <div
          data-test-template-json
          {{codeMirror
            theme="hashi"
            mode="ruby"
            autofocus=false
            content=this.template
            onUpdate=this.updateTemplate
          }}
        ></div>
      </label>
    </div>
  </template>
}
