/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';
import { diffLines } from 'diff';

@classic
@tagName('')
export default class JobDiffTemplate extends Component {
  get diffChunks() {
    return diffLines(this.field.Old, this.field.New);
  }
}
