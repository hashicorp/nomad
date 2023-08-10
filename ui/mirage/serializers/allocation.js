/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import { arrToObj } from '../utils';

export default ApplicationSerializer.extend({
  embed: true,
  include: ['taskStates', 'taskResources'],

  serialize() {
    var json = ApplicationSerializer.prototype.serialize.apply(this, arguments);
    if (json instanceof Array) {
      json.forEach(serializeAllocation);
    } else {
      serializeAllocation(json);
    }
    return json;
  },
});

function serializeAllocation(allocation) {
  allocation.TaskStates = allocation.TaskStates.reduce(arrToObj('Name'), {});
  const { Ports, Networks } = allocation.TaskResources[0]
    ? allocation.TaskResources[0].Resources
    : {};
  allocation.AllocatedResources = {
    Shared: { Ports, Networks },
    Tasks: allocation.TaskResources.map(({ Name, Resources }) => ({ Name, ...Resources })).reduce(
      arrToObj('Name'),
      {}
    ),
  };
}
