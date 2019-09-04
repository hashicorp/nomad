import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentArray } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  upstreams: fragmentArray('sidecar-proxy-upstream'),
});
