/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';
import LineDiff from 'line-diff';

@classic
@tagName('')
export default class JobDiffTemplate extends Component {
  get diff() {
    return new LineDiff(this.field.Old, this.field.New).toString();
  }
}
