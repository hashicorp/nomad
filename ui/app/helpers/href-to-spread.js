import Ember from 'ember';
import hrefTo from 'ember-href-to/helpers/href-to';

const { Helper } = Ember;

/**
 * Href-to Spread
 *
 * Usage: {{href-to-spread hrefToPositionalParamsAsArray query=whatever}}
 *
 * Does the same thing as href-to but takes an array of arguments instead of a static list.
 * This way arguments can be managed in js and provided to the template.
 */
export default Helper.extend({
  compute([params], options = {}) {
    return hrefTo.create().compute.call(this, params, options);
  },
});
