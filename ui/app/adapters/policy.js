/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { default as ApplicationAdapter, namespace } from './application';
import classic from 'ember-classic-decorator';

@classic
export default class PolicyAdapter extends ApplicationAdapter {
  namespace = namespace + '/acl';

  urlForCreateRecord(_modelName, model) {
    return this.urlForUpdateRecord(model.attr('name'), 'policy');
  }

  urlForDeleteRecord(id) {
    return this.urlForUpdateRecord(id, 'policy');
  }
}
