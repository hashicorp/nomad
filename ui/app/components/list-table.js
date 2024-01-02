/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { computed } from '@ember/object';
import { computed as overridable } from 'ember-overridable-computed';
import { classNames, tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('table')
@classNames('table')
export default class ListTable extends Component {
  @overridable(() => []) source;

  // Plan for a future with metadata (e.g., isSelected)
  @computed('source.{[],isFulfilled}')
  get decoratedSource() {
    return (this.source || []).map((row) => ({
      model: row,
    }));
  }
}
