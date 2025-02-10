/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import ApplicationAdapter from './application';
import classic from 'ember-classic-decorator';

@classic
export default class VersionTagAdapter extends ApplicationAdapter {
  urlForCreateRecord(_modelName, model) {
    const tagName = model.attr('name');
    const jobName = model.attr('jobName');
    return `${this.buildURL()}/job/${jobName}/versions/${tagName}/tag`;
  }

  async deleteTag(jobName, tagName) {
    let deletion = this.ajax(
      this.urlForDeleteRecord(jobName, tagName),
      'DELETE'
    );
    return deletion;
  }

  urlForDeleteRecord(jobName, tagName) {
    return `${this.buildURL()}/job/${jobName}/versions/${tagName}/tag`;
  }
}
