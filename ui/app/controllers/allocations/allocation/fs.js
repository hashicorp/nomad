/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { computed } from '@ember/object';

export default class FsController extends Controller {
  queryParams = [
    {
      sortProperty: 'sort',
    },
    {
      sortDescending: 'desc',
    },
  ];

  sortProperty = 'Name';
  sortDescending = false;

  path = null;
  allocation = null;
  directoryEntries = null;
  isFile = null;
  stat = null;

  @computed('path')
  get pathWithLeadingSlash() {
    const path = this.path;

    if (path.startsWith('/')) {
      return path;
    } else {
      return `/${path}`;
    }
  }
}
