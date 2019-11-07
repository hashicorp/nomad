import Ember from 'ember';
import fetch from 'fetch';
import config from '../config/environment';

// The ember-fetch polyfill does not provide streaming
// Additionally, Mirage/Pretender does not support fetch
const mirageEnabled =
  config.environment !== 'production' &&
  config['ember-cli-mirage'] &&
  config['ember-cli-mirage'].enabled !== false;

const fetchToUse = Ember.testing || mirageEnabled ? fetch : window.fetch || fetch;

export default fetchToUse;
