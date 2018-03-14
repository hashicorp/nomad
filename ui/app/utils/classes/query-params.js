// Copied from source since it isn't exposed to import
// https://github.com/emberjs/ember.js/blob/v2.18.2/packages/ember-routing/lib/system/query_params.js
import EmberObject from '@ember/object';

const QueryParams = EmberObject.extend({
  isQueryParams: true,
  values: null,
});

export const qpBuilder = values => QueryParams.create({ values });

export default QueryParams;
