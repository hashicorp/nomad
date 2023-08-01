import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  keyForAttribute(str) {
    if (str === 'config' || str === 'member') {
      return str;
    }
    return ApplicationSerializer.prototype.keyForAttribute.apply(this, arguments);
  },
});
