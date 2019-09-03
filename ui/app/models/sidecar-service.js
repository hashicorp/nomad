import Fragment from 'ember-data-model-fragments/fragment';
import { fragment } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  proxy: fragment('sidecar-proxy'),
});
