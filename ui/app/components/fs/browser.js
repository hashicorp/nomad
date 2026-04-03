/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { get } from '@ember/object';
import { compare } from '@ember/utils';
import Component from '@ember/component';
import { computed } from '@ember/object';
import { filterBy } from '@ember/object/computed';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('')
export default class Browser extends Component {
  model = null;

  @computed('model.allocation')
  get allocation() {
    if (this.model.allocation) {
      return this.model.allocation;
    } else {
      return this.model;
    }
  }

  @computed('model.allocation')
  get taskState() {
    if (this.model.allocation) {
      return this.model;
    }

    return undefined;
  }

  @computed('taskState')
  get type() {
    if (this.taskState) {
      return 'task';
    } else {
      return 'allocation';
    }
  }

  @filterBy('directoryEntries', 'IsDir') directories;
  @filterBy('directoryEntries', 'IsDir', false) files;

  @computed(
    'directories',
    'directoryEntries.[]',
    'files',
    'sortDescending',
    'sortProperty'
  )
  get sortedDirectoryEntries() {
    const sortProperty = this.sortProperty;

    const directorySortProperty =
      sortProperty === 'Size' ? 'Name' : sortProperty;

    const sortedDirectories = [...this.directories].sort((a, b) =>
      compare(get(a, directorySortProperty), get(b, directorySortProperty))
    );
    const sortedFiles = [...this.files].sort((a, b) =>
      compare(get(a, sortProperty), get(b, sortProperty))
    );

    const sortedDirectoryEntries = sortedDirectories.concat(sortedFiles);

    if (this.sortDescending) {
      return sortedDirectoryEntries.reverse();
    } else {
      return sortedDirectoryEntries;
    }
  }
}
