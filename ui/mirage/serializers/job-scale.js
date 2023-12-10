/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import { arrToObj } from '../utils';

export default ApplicationSerializer.extend({
  embed: true,
  include: ['taskGroupScales'],

  serialize() {
    var json = ApplicationSerializer.prototype.serialize.apply(this, arguments);
    if (json instanceof Array) {
      json.forEach(serializeJobScale);
    } else {
      serializeJobScale(json);
    }
    return json;
  },
});

function serializeJobScale(jobScale) {
  jobScale.TaskGroups = jobScale.TaskGroupScales.reduce(arrToObj('Name'), {});
}
