import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  embed: true,
  include: ['task_states', 'task_resources'],

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
  allocation.TaskResources = allocation.TaskResources.reduce(arrToObj('Name', 'Resources'), {});
}

function arrToObj(prop, alias = '') {
  return (obj, element) => {
    const name = element[prop];
    delete element[prop];

    obj[name] = alias ? element[alias] : element;
    return obj;
  };
}
