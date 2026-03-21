/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Watchable from './watchable';
import codesForError from '../utils/codes-for-error';

export default class NamespaceAdapter extends Watchable {
  findRecord(store, modelClass, id) {
    return super.findRecord(...arguments).catch((error) => {
      const errorCodes = codesForError(error);
      if (errorCodes.includes('501')) {
        return { Name: id };
      }
    });
  }

  urlForCreateRecord(_modelName, model) {
    return this.urlForUpdateRecord(model.attr('name'), 'namespace');
  }

  urlForDeleteRecord(id) {
    return this.urlForUpdateRecord(id, 'namespace');
  }
}
