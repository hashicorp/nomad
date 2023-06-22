/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import ApplicationAdapter from './application';
import classic from 'ember-classic-decorator';
import { pluralize } from 'ember-inflector';

@classic
export default class NodePoolAdapter extends ApplicationAdapter {
  urlForFindAll(modelName) {
    let [relationshipResource, resource] = modelName.split('-');
    resource = pluralize(resource);
    return `/v1/${relationshipResource}/${resource}`;
  }
}
