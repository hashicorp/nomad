/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Watchable from './watchable';
import codesForError from '../utils/codes-for-error';
import classic from 'ember-classic-decorator';

@classic
export default class NamespaceAdapter extends Watchable {
  findRecord(store, modelClass, id) {
    return super.findRecord(...arguments).catch((error) => {
      const errorCodes = codesForError(error);
      if (errorCodes.includes('501')) {
        return { Name: id };
      }
    });
  }
}
