/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationAdapter from './application';
import classic from 'ember-classic-decorator';

@classic
export default class EvaluationAdapter extends ApplicationAdapter {
  handleResponse(_status, headers) {
    const result = super.handleResponse(...arguments);
    result.meta = { nextToken: headers['x-nomad-nexttoken'] };
    return result;
  }

  urlForFindRecord(_id, _modelName, snapshot) {
    const namespace = snapshot.attr('namespace') || 'default';
    const baseURL = super.urlForFindRecord(...arguments);
    const url = `${baseURL}?namespace=${namespace}`;

    if (snapshot.adapterOptions?.related) {
      return `${url}&related=true`;
    }
    return url;
  }
}
