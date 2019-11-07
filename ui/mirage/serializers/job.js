import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  embed: true,
  include: ['task_groups', 'job_summary'],

  serialize() {
    var json = ApplicationSerializer.prototype.serialize.apply(this, arguments);
    if (json instanceof Array) {
      json.forEach(serializeJob);
    } else {
      serializeJob(json);
    }
    return json;
  },
});

function serializeJob(job) {
  job.TaskGroups.forEach(group => {
    if (group.Services.length === 0) {
      group.Services = null;
    }
  });
}
