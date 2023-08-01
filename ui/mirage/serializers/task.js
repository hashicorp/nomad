import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  embed: true,
  include: ['services'],
});
