import fetch from 'fetch';

// The ember-fetch polyfill does not provide streaming
const fetchToUse = window.fetch || fetch;

export default fetchToUse;
