import { equal } from '@ember/object/computed';
import Service from '@ember/service';
import { get } from '@ember/object';
import config from '../config/environment';

export default Service.extend({
  unknownProperty(path) {
    return get(config, path);
  },

  isDev: equal('environment', 'development'),
  isProd: equal('environment', 'production'),
  isTest: equal('environment', 'test'),
});
