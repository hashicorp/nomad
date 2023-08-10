/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import { arrToObj } from '../utils';

export default ApplicationSerializer.extend({
  embed: true,
  include: ['deploymentTaskGroupSummaries'],

  serialize() {
    var json = ApplicationSerializer.prototype.serialize.apply(this, arguments);
    if (json instanceof Array) {
      json.forEach(serializeDeployment);
    } else {
      serializeDeployment(json);
    }
    return json;
  },
});

function serializeDeployment(deployment) {
  deployment.TaskGroups = deployment.DeploymentTaskGroupSummaries.reduce(arrToObj('Name'), {});
}
