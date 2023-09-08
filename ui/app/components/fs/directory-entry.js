/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { computed } from '@ember/object';
import { isEmpty } from '@ember/utils';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('')
export default class DirectoryEntry extends Component {
  allocation = null;
  taskState = null;

  @computed('path', 'entry.Name')
  get pathToEntry() {
    const pathWithNoLeadingSlash = this.path.replace(/^\//, '');
    const name = encodeURIComponent(this.get('entry.Name'));

    if (isEmpty(pathWithNoLeadingSlash)) {
      return name;
    } else {
      return `${pathWithNoLeadingSlash}/${name}`;
    }
  }
}
