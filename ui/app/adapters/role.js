/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: MPL-2.0
 */

import { default as ApplicationAdapter, namespace } from './application';
import { singularize } from 'ember-inflector';
export default class RoleAdapter extends ApplicationAdapter {
  namespace = namespace + '/acl';

  urlForCreateRecord(modelName) {
    let baseUrl = this.buildURL(modelName);
    return singularize(baseUrl);
  }

  urlForDeleteRecord(id) {
    return this.urlForUpdateRecord(id, 'role');
  }
}
