import Ember from 'ember';
import config from '../config/environment';

const { Service, get } = Ember;

export default Service.extend({
  unknownProperty(path) {
    return get(config, path);
  },
});
