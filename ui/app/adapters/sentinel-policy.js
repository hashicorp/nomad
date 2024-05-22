/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { default as ApplicationAdapter } from './application';
import classic from 'ember-classic-decorator';

@classic
export default class SentinelPolicyAdapter extends ApplicationAdapter {
  pathForType = () => 'sentinel/policies';

  //   namespace = namespace + '/acl';
  urlForCreateRecord(_modelName, model) {
    return this.urlForUpdateRecord(model.attr('name'), 'sentinel/policy');
  }

  urlForFindRecord(id) {
    return '/v1/sentinel/policy/' + id;
  }

  urlForDeleteRecord(id) {
    return '/v1/sentinel/policy/' + id;
  }
}
