import Ember from 'ember';
import fetch from 'fetch';

// The ember-fetch polyfill does not provide streaming
// Additionally, Mirage/Pretender does not support fetch
const fetchToUse = Ember.testing ? fetch : window.fetch || fetch;

export default fetchToUse;
