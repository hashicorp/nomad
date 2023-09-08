/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class MetadataKvComponent extends Component {
  @tracked editing = false;
  @tracked value = this.args.value;
  get prefixedKey() {
    return this.args.prefix
      ? `${this.args.prefix}.${this.args.key}`
      : this.args.key;
  }

  @action onEdit(event) {
    if (event.key === 'Escape') {
      this.editing = false;
    }
  }
}
