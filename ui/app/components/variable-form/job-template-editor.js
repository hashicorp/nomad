/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import { action } from '@ember/object';
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';

export default class JobTemplateEditor extends Component {
  @tracked description;
  @tracked template;
  @action
  establishKeyValues() {
    this.description = this.args.keyValues.findBy('key', 'description')?.value;
    this.template = this.args.keyValues.findBy('key', 'template')?.value;
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
