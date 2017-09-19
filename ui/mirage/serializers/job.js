import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  embed: true,
  include: ['task_groups', 'job_summary'],
});
