/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { default as ApplicationAdapter, namespace } from './application';
import classic from 'ember-classic-decorator';

// TODO: Nomitch - Update this
@classic
export default class SentinelPolicyAdapter extends ApplicationAdapter {
  pathForType = () => 'sentinel/policies';

  //   namespace = namespace + '/acl';
  //   urlForCreateRecord(_modelName, model) {
  //     return this.urlForUpdateRecord(model.attr('name'), 'policy');
  //   }
  //   urlForDeleteRecord(id) {
  //     return this.urlForUpdateRecord(id, 'policy');
  //   }
}
