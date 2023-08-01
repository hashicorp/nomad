import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  serialize() {
    var json = ApplicationSerializer.prototype.serialize.apply(this, arguments);
    return [].concat(json).mapBy('ID');
  },
});
