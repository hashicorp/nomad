/**
 * Copyright IBM Corp. 2015, 2025
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
    const namespace = model.attr('jobNamespace');
    return `${this.buildURL()}/job/${jobName}/versions/${tagName}/tag?namespace=${namespace}`;
  }

  async deleteTag(namespace, jobName, tagName) {
    let deletion = this.ajax(
      this.urlForDeleteRecord(namespace, jobName, tagName),
      'DELETE'
    );
    return deletion;
  }

  urlForDeleteRecord(namespace, jobName, tagName) {
    return `${this.buildURL()}/job/${jobName}/versions/${tagName}/tag?namespace=${namespace}`;
  }
}
