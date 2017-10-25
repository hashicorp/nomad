import Ember from 'ember';
import config from '../config/environment';

const { Service, get, computed } = Ember;

export default Service.extend({
  unknownProperty(path) {
    return get(config, path);
  },

  isDev: computed.equal('environment', 'development'),
  isProd: computed.equal('environment', 'production'),
  isTest: computed.equal('environment', 'test'),
});
