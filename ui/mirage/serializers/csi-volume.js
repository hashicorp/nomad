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
      json.forEach(serializeVolume);
    } else {
      serializeVolume(json);
    }
    return json;
  },
});

function serializeVolume(volume) {
  volume.WriteAllocs = groupBy(volume.WriteAllocs, 'ID');
  volume.ReadAllocs = groupBy(volume.ReadAllocs, 'ID');
}
