/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';

const groupBy = (list, attr) => {
  return list.reduce((group, item) => {
    group[item[attr]] = item;
    return group;
  }, {});
};

export default ApplicationSerializer.extend({
  embed: true,
  include: ['writeAllocs', 'readAllocs', 'allocations'],

  serialize() {
    var json = ApplicationSerializer.prototype.serialize.apply(this, arguments);
    if (json instanceof Array) {
      json.forEach(serializeVolumeFromArray);
    } else {
      serializeVolume(json);
    }
    return json;
  },
});

function serializeVolumeFromArray(volume) {
  volume.CurrentWriters = volume.WriteAllocs.length;
  delete volume.WriteAllocs;

  volume.CurrentReaders = volume.ReadAllocs.length;
  delete volume.ReadAllocs;
}

function serializeVolume(volume) {
  volume.WriteAllocs = groupBy(volume.WriteAllocs, 'ID');
  volume.ReadAllocs = groupBy(volume.ReadAllocs, 'ID');
}
