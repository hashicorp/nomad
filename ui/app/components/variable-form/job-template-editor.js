/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import { get } from '@ember/object';
import { action } from '@ember/object';
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';

export default class JobTemplateEditor extends Component {
  @tracked description;
  @tracked template;
  @action
  establishKeyValues() {
    this.description = this.args.keyValues.find(
      (item) => get(item, 'key') === 'description'
    )?.value;
    this.template = this.args.keyValues.find(
      (item) => get(item, 'key') === 'template'
    )?.value;
  }

  @action
  updateDescription(event) {
    this.args.updateKeyValue('description', event.target.value);
  }
  @action
  updateTemplate(value) {
    this.args.updateKeyValue('template', value);
  }
}
