/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { computed } from '@ember/object';
import { computed as overridable } from 'ember-overridable-computed';
import moment from 'moment';
import { classNames, tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('ol')
@classNames('timeline')
export default class JobDeploymentsStream extends Component {
  @overridable(() => []) deployments;

  @computed('deployments.@each.versionSubmitTime')
  get sortedDeployments() {
    return this.deployments.sortBy('versionSubmitTime').reverse();
  }

  @computed('sortedDeployments.@each.version')
  get annotatedDeployments() {
    const deployments = this.sortedDeployments;
    return deployments.map((deployment, index) => {
      const meta = {};

      if (index === 0) {
        meta.showDate = true;
      } else {
        const previousDeployment = deployments.objectAt(index - 1);
        const previousSubmitTime = previousDeployment.get('version.submitTime');
        const submitTime = deployment.get('submitTime');
        if (
          submitTime &&
          previousSubmitTime &&
          moment(previousSubmitTime)
            .startOf('day')
            .diff(moment(submitTime).startOf('day'), 'days') > 0
        ) {
          meta.showDate = true;
        }
      }

      return { deployment, meta };
    });
  }
}
