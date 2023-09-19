/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// @ts-check
import { default as ApplicationAdapter, namespace } from './application';
import classic from 'ember-classic-decorator';
import { singularize } from 'ember-inflector';
@classic
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
